package ytdlp

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	ok := []struct{ in, site, url string }{
		{"https://www.youtube.com/watch?v=synthetic01", SiteYouTube, "https://www.youtube.com/watch?v=synthetic01"},
		{"  https://music.youtube.com/watch?v=synthetic01&list=PLsynthetic" + "#t=3  ", SiteYouTube, "https://music.youtube.com/watch?v=synthetic01&list=PLsynthetic"},
		{"https://m.youtube.com/watch?v=synthetic01", SiteYouTube, "https://www.youtube.com/watch?v=synthetic01"},
		{"http://youtu.be/synthetic01", SiteYouTube, "https://youtu.be/synthetic01"},
		{"youtu.be/synthetic01", SiteYouTube, "https://youtu.be/synthetic01"},
		{"HTTPS://WWW.YOUTUBE.COM/watch?v=synthetic01", SiteYouTube, "https://www.youtube.com/watch?v=synthetic01"},
		{"https://www.bilibili.com/video/BV1Synthetic?p=2", SiteBilibili, "https://www.bilibili.com/video/BV1Synthetic?p=2"},
		{"https://m.bilibili.com/video/BV1Synthetic", SiteBilibili, "https://www.bilibili.com/video/BV1Synthetic"},
		// bilibili's share text: the link is picked out of it.
		{"【Synthetic title-哔哩哔哩】 https://b23.tv/Synth01", SiteBilibili, "https://b23.tv/Synth01"},
	}
	for _, c := range ok {
		got, err := ParseURL(c.in)
		if err != nil {
			t.Errorf("ParseURL(%q): %v", c.in, err)
			continue
		}
		if got.Site != c.site || got.URL != c.url {
			t.Errorf("ParseURL(%q) = %+v, want %s %s", c.in, got, c.site, c.url)
		}
	}
	if got, _ := ParseURL("https://b23.tv/Synth01"); !got.Short() {
		t.Error("b23.tv link not marked short")
	}
	if got, _ := ParseURL("https://www.bilibili.com/video/BV1Synthetic"); got.Short() {
		t.Error("full bilibili link marked short")
	}

	bad := []string{
		"",
		"not a link",
		"https://evil.example.com/watch?v=synthetic01",
		"https://www.youtube.com.example.com/watch?v=synthetic01",
		"https://live.bilibili.com/1", // live streams never end
		// Built from parts so no URL with credentials or a port is spelled out in the source.
		(&url.URL{Scheme: "https", User: url.UserPassword("user", "pw"), Host: "www.youtube.com", Path: "/watch"}).String(),
		(&url.URL{Scheme: "https", Host: "www.youtube.com:8443", Path: "/watch"}).String(),
		"ftp://www.youtube.com/watch?v=synthetic01",
		"file:///etc/passwd",
		"http://192.0.2.1/",
		"https://www.youtube.com/watch?v=" + strings.Repeat("a", maxURLLength),
	}
	for _, in := range bad {
		if got, err := ParseURL(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseURL(%q) = %+v, %v; want ErrInvalid", in, got, err)
		}
	}
}
