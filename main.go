package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kolo/xmlrpc"
	"github.com/thoj/go-ircevent"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app           = kingpin.New("apoller", "Autoleecher for APOLLO")
	username      = app.Flag("username", "Apollo username").Required().String()
	passkey       = app.Flag("passkey", "Apollo torrent pass key").Required().String()
	authkey       = app.Flag("authkey", "Apollo torrent auth key").Required().String()
	ircKey        = app.Flag("irc_key", "Apollo IRC key").Required().String()
	ircNick       = app.Flag("irc_nick", "IRC nick").Required().String()
	ircUser       = app.Flag("irc_user", "IRC user").Default("apoller").String()
	ircServer     = app.Flag("irc_server", "IRC server").Default("irc.apollo.rip:7000").String()
	ircSSL        = app.Flag("irc_ssl", "Use SSL for IRC connection").Default("true").Bool()
	rtorrentURL   = app.Flag("rtorrent_url", "URL to communicate with rTorrent").Required().String()
	filterYears   = app.Flag("filter_years", "Filter downloads to releases from these years").Ints()
	filterTags    = app.Flag("filter_tags", "Filter downloads to releases with these tags").Strings()
	filterFormats = app.Flag("filter_formats", "Filter downloads to releases with these formats").Strings()
)

type announce struct {
	time   time.Time
	name   string
	format string
	page   string
	url    string
	artist string
	title  string
	year   int
	kind   string
	tags   []string
	id     int
}

var announceRegexp = regexp.MustCompile("^\x02TORRENT:\x02 \x0303(.+?)\x03 - \x0310(.+?)\x03\x03 - \x0312(.+?)\x03 - \x0304(.+?)\x03 / \x0304(.+?)\x03$")
var nameRegexp = regexp.MustCompile("^(.+?) - (.+?) \\[([0-9]+)\\] \\[(.+?)\\]$")

func parseAnnounce(s string) (*announce, error) {
	m1 := announceRegexp.FindStringSubmatch(s)
	if m1 == nil {
		return nil, fmt.Errorf("announce didn't match regexp")
	}
	name, format, tagString, page, url := m1[1], m1[2], m1[3], m1[4], m1[5]

	m2 := nameRegexp.FindStringSubmatch(name)
	if m2 == nil {
		return nil, fmt.Errorf("couldn't parse name into parts")
	}
	artist, title, yearString, kind := m2[1], m2[2], m2[3], m2[4]

	year, err := strconv.ParseUint(yearString, 10, 32)
	if err != nil {
		return nil, err
	}

	tags := strings.Split(tagString, ",")
	for i, v := range tags {
		tags[i] = strings.TrimSpace(v)
	}

	return &announce{
		time:   time.Now(),
		name:   name,
		format: format,
		page:   page,
		url:    url,
		artist: artist,
		title:  title,
		year:   int(year),
		kind:   kind,
		tags:   tags,
	}, nil
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	r, err := xmlrpc.NewClient(*rtorrentURL, nil)
	if err != nil {
		panic(err)
	}

	c := irc.IRC(*ircNick, *ircUser)

	c.UseTLS = *ircSSL
	c.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := c.Connect(*ircServer); err != nil {
		panic(err)
	}

	c.AddCallback("001", func(e *irc.Event) {
		c.Privmsgf("APOLLO", "enter #announce %s %s", *username, *ircKey)
	})

	c.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Code == "PRIVMSG" && e.Source == "APOLLO!APOLLO@apollo.apollo.rip" && len(e.Arguments) >= 2 && strings.EqualFold(e.Arguments[0], "#announce") {
			a, err := parseAnnounce(e.Arguments[1])
			if err != nil {
				fmt.Printf("failed to parse %q\n", e.Arguments[1])
				return
			}

			if len(*filterYears) > 0 {
				foundYear := false

				for _, year := range *filterYears {
					if a.year == year {
						foundYear = true
						break
					}
				}

				if !foundYear {
					return
				}
			}

			if len(*filterTags) > 0 {
				foundTag := false

			outer:
				for _, tag := range *filterTags {
					for _, t := range a.tags {
						if t == tag {
							foundTag = true
							break outer
						}
					}
				}

				if !foundTag {
					return
				}
			}

			if len(*filterFormats) > 0 {
				foundFormat := false

				for _, format := range *filterFormats {
					if a.format == format {
						foundFormat = true
						break
					}
				}

				if !foundFormat {
					return
				}
			}

			downloadURL := a.url + fmt.Sprintf("&authkey=%s&torrent_pass=%s", *authkey, *passkey)

			var rc int
			if err := r.Call("load_start", []string{downloadURL}, &rc); err != nil {
				fmt.Printf("couldn't download torrent: %s\n", err.Error())
				return
			}

			if rc != 0 {
				fmt.Printf("got strange response from rtorrent: %d\n", rc)
				return
			}
		}
	})

	c.Loop()
}
