package hindsight

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

// lock pins every Python package Hindsight runs on to one version and its
// hashes. It leaves out claude-agent-sdk and github-copilot-sdk, which are
// proprietary and which Hindsight imports only when asked to, and whatever
// only they need. It was made with:
//
//	printf 'hindsight-api-slim[local-onnx,embedded-db]==1.2.1\n' >in.txt
//	printf 'claude-agent-sdk\ngithub-copilot-sdk\n' >excludes.txt
//	uv pip compile in.txt --excludes excludes.txt --generate-hashes \
//		--python-version 3.13 --python-platform x86_64-unknown-linux-gnu \
//		--no-header -o hindsight.lock
//
//go:embed hindsight.lock
var lock []byte

// pythonVersion is the Python the lock was made for; setup gives Hindsight
// its own copy of it.
const pythonVersion = "3.13"

// lockSum names the lock an environment was installed from, so a Rashin
// that ships another lock asks for setup again.
func lockSum() string {
	sum := sha256.Sum256(lock)
	return hex.EncodeToString(sum[:])
}
