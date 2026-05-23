package main

import "net/url"

type urlT = url.URL

func parseURL(s string) *urlT {
	u, _ := url.Parse(s)
	return u
}
