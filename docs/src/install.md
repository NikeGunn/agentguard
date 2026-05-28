# Install

## macOS / Linux

```bash
curl -fsSL https://agentguard.space/install | sh
```

This installs to `~/.agentguard/bin/agentguard` and adds it to your
`PATH` if it isn't already.

## Windows (PowerShell)

```powershell
iwr -useb https://agentguard.space/install.ps1 | iex
```

Installs to `%USERPROFILE%\.agentguard\bin\agentguard.exe` and appends
the directory to the user `PATH`.

## From source

```bash
git clone https://github.com/nikegunn/agentguard
cd agentguard
go install ./cmd/agentguard
```

Requires Go 1.23 or newer.

## Verifying a release

Every tagged release is signed with [cosign](https://docs.sigstore.dev/cosign/overview/)
keyless OIDC via GitHub Actions. To verify a downloaded archive:

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature   checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/nikegunn/agentguard' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum -c checksums.txt --ignore-missing
```

## Uninstall

```bash
agentguard uninstall          # restores every patched config byte-for-byte
agentguard uninstall --purge  # also removes ~/.agentguard
```
