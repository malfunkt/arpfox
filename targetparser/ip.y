%{

package targetparser

import (
    "bytes"
    "encoding/binary"
    "log"
    "net"
    "strconv"
    "unicode/utf8"

    "github.com/pkg/errors"
)

type AddressRange struct {
    Min net.IP
    Max net.IP
}

type octetRange struct {
    min byte
    max byte
}

%}

%union {
    num         byte
    octRange    octetRange
    addrRange   AddressRange
}

%token  <num> NUM
%token  <num>  MASK
%type   <addrRange> address target
%type   <octRange>   term octet_range

%%

target:     address '/' NUM
                {
                    mask := net.CIDRMask(int($3), 32)
                    min := $1.Min.Mask(mask)
                    maxInt := binary.BigEndian.Uint32([]byte(min)) +
                                0xffffffff -
                                binary.BigEndian.Uint32([]byte(mask))
                    maxBytes := make([]byte, 4)
                    binary.BigEndian.PutUint32(maxBytes, maxInt)
                    maxBytes = maxBytes[len(maxBytes)-4:]
                    max := net.IP(maxBytes)
                    $$ = AddressRange {
                        Min: min,
                        Max: max,
                    }
                    iplex.(*ipLex).output = $$
                }
      |     address
                {
                    $$ = $1
                    iplex.(*ipLex).output = $$
                }

address:    term '.' term '.' term '.' term
                {
                    $$ = AddressRange {
                        Min: net.IPv4($1.min, $3.min, $5.min, $7.min),
                        Max: net.IPv4($1.max, $3.max, $5.max, $7.max),
                    }
                }

term:   NUM       { $$ = octetRange { $1, $1 } }
    |   '*'         { $$ = octetRange { 0, 255 } }
    |   octet_range { $$ = $1 }

octet_range:    NUM '-' NUM { $$ = octetRange { $1, $3 } }

%%

const eof = 0

type ipLex struct {
    line    []byte
    peek    rune
    output  AddressRange
    err     error
}

func (ip *ipLex) Lex(yylval *ipSymType) int {
    for {
        c := ip.next()
        switch c {
        case eof:
            return eof
        case '.', '-', '/', '*':
            return int(c)
        case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
            return ip.byte(c, yylval)
        case ' ', '\t', '\n', '\r':
        default:
        }
    }
}

func (ip *ipLex) byte(c rune, yylval *ipSymType) int {
    add := func(b *bytes.Buffer, c rune) {
        if _, err := b.WriteRune(c); err != nil {
            log.Fatalf("WriteRune: %s", err)
        }
    }
    var b bytes.Buffer
    add(&b, c)
    L: for {
        c = ip.next()
        switch c {
        case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
            add(&b, c)
        default:
            break L
        }
    }
    if c != eof {
        ip.peek = c
    }
    octet, err := strconv.ParseUint(b.String(), 10, 32)
    if err != nil {
        log.Printf("badly formatted octet")
        return eof
    }
    yylval.num = byte(octet)
    return NUM
}

func (ip *ipLex) next() rune {
    if ip.peek != eof {
        r := ip.peek
        ip.peek = eof
        return r
    }
    if len(ip.line) == 0 {
        return eof
    }
    c, size := utf8.DecodeRune(ip.line)
    ip.line = ip.line[size:]
    if c == utf8.RuneError && size == 1 {
        log.Print("invalid utf8")
        return ip.next()
    }
    return c
}

func (ip *ipLex) Error(s string) {
    ip.err = errors.New(s)
}

func Parse(in string) (*AddressRange, error) {
    lex := &ipLex{line: []byte(in)}
    errCode := ipParse(lex)
    if errCode != 0 || lex.err != nil {
        return nil, errors.Wrap(lex.err, "could not parse target")
    }
    return &lex.output, nil
}
