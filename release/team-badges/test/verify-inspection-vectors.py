"""Verify immutable signed protocol vectors with Python; no network or live keys."""
import base64
import hashlib
import json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey


def verify_vectors():
    fixture = json.loads(Path(__file__).with_name("inspection-vectors.json").read_text())
    for vector in fixture["vectors"]:
        raw = json.dumps(vector["payload"], ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()
        assert raw.hex() == vector["canonicalHex"]
        assert hashlib.sha256(raw).hexdigest() == vector["payloadSha"]
        message = vector["domain"].encode() + b"\0" + raw
        assert message.hex() == vector["messageHex"]
        assert base64.b64decode(vector["envelope"]["payload"], validate=True) == raw
        public = Ed25519PublicKey.from_public_bytes(base64.b64decode(vector["publicKey"], validate=True))
        public.verify(base64.b64decode(vector["envelope"]["sig"], validate=True), message)
    print(f"Verified {len(fixture['vectors'])} Python canonical/signature vectors")


if __name__ == "__main__":
    verify_vectors()
