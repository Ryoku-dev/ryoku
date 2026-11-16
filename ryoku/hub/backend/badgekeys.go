package main

import "encoding/base64"

var badgeIssuerKeys = map[byte][]byte{
	1: badgePublicKey("tlIg8prWfO0dMfd9IO3qxklONGHbSq7w3bUiu6QWuro="),
	2: badgePublicKey("TtnSdWqRm4UPQ4JXa5IG8yBKp/xbxTudgO/C/AeBnUg="),
}

func badgePublicKey(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }
