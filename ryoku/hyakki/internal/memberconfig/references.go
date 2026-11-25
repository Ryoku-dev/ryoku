package memberconfig

import (
	"net/url"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

type resourceLimits struct {
	nodes int
	bytes int
}

func (limits *resourceLimits) add(depth int, value string, key bool) error {
	limits.nodes++
	limits.bytes += len(value)
	if depth > MaxDepth || limits.nodes > MaxNodes || len(value) > MaxScalarBytes || limits.bytes > MaxContentBytes || key && len(value) > MaxKeyBytes {
		return bad("Member configuration exceeds its depth (32), node (8192), scalar (64 KiB), key (1 KiB), or aggregate (4 MiB) limit.")
	}
	if !utf8.ValidString(value) {
		return bad("Member configuration must contain valid UTF-8.")
	}
	return nil
}

// CheckBounds applies the same finite representation limits to ordinary forms,
// YAML and exports. Reserve 16 nodes and 4 KiB for the version/team envelope,
// numeric scalars and growth when short vault references become export masks.
// All member fields, maps and lists use the JSON-tag schema.
// Runtime's independent 64-argument/16-KiB limits remain unchanged.
func CheckBounds(member protocol.Member) error {
	limits, err := canonicalBounds(member)
	if err != nil {
		return err
	}
	// Even maximal depth indentation and JSON/YAML escaping fit this loose
	// upper bound. Ordinary small forms need no marshal or YAML allocation.
	wireLimit := MaxBytes - 6*MaxKeyBytes - 64
	if (limits.bytes+limits.nodes)*128 <= wireLimit {
		return nil
	}
	// Only larger configurations need exact rendering. Leave room for the
	// actual team ID and subsequent revision growth without knowing the team.
	_, err = encodeDocument("", member, wireLimit)
	return err
}

func canonicalBounds(member protocol.Member) (resourceLimits, error) {
	limits := resourceLimits{nodes: 16, bytes: 4096}
	err := checkValueBounds(reflect.ValueOf(member), documentSchema.names["member"], 1, &limits)
	return limits, err
}

func checkValueBounds(value reflect.Value, typed *schema, depth int, limits *resourceLimits) error {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return limits.add(depth, "", false)
		}
		value = value.Elem()
	}
	scalar := ""
	if value.Kind() == reflect.String {
		scalar = value.String()
	}
	if err := limits.add(depth, scalar, false); err != nil {
		return err
	}
	switch typed.kind {
	case reflect.Struct:
		for _, field := range typed.fields {
			child := value.Field(field.index)
			if field.omitEmpty {
				switch child.Kind() {
				case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
					if child.Len() == 0 {
						continue
					}
				default:
					if child.IsZero() {
						continue
					}
				}
			}
			if err := limits.add(depth+1, field.name, true); err != nil {
				return err
			}
			if err := checkValueBounds(child, field.value, depth+1, limits); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if err := limits.add(depth+1, iter.Key().String(), true); err != nil {
				return err
			}
			if err := checkValueBounds(iter.Value(), typed.element, depth+1, limits); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if err := checkValueBounds(value.Index(i), typed.element, depth+1, limits); err != nil {
				return err
			}
		}
	}
	return nil
}

// CheckReferences uses metadata only. The lookup must never reveal a value or
// unlock a keyring; runtime exports keep their separate process-read consent.
func CheckReferences(team string, member protocol.Member, lookup func(string) (protocol.Credential, bool)) error {
	if member.Runtime == nil {
		return nil
	}
	check := func(ref, use string) (protocol.Credential, error) {
		if ref == Mask || strings.TrimSpace(ref) == "" || len(ref) > 200 || strings.ContainsRune(ref, 0) {
			return protocol.Credential{}, bad("A runtime reference must name an existing vault entry.")
		}
		credential, found := lookup(ref)
		if !found || credential.Use != use || !slices.Contains(credential.Teams, team) {
			return protocol.Credential{}, protocol.Errorf(protocol.CodeForbidden, "A configured vault reference is unavailable for this team and use; grant it in Vault settings first.")
		}
		return credential, nil
	}
	for _, ref := range member.Runtime.SecretEnv {
		if _, err := check(ref, protocol.CredentialRuntime); err != nil {
			return err
		}
	}
	if member.Runtime.Credential != "" {
		credential, err := check(member.Runtime.Credential, protocol.CredentialHeader)
		if err != nil {
			return err
		}
		endpoint, err := url.Parse(member.Runtime.URL)
		if err != nil {
			return protocol.Errorf(protocol.CodeForbidden, "The webhook endpoint is outside its vault credential's host grant.")
		}
		host := strings.ToLower(endpoint.Hostname())
		if port := endpoint.Port(); port != "" && !(endpoint.Scheme == "https" && port == "443") && !(endpoint.Scheme == "http" && port == "80") {
			host += ":" + port
		}
		if !hostrule.Allows(credential.Hosts, host) {
			return protocol.Errorf(protocol.CodeForbidden, "The webhook endpoint is outside its vault credential's host grant.")
		}
	}
	return nil
}
