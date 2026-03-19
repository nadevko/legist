{
  writeScript,
  go-swag,
  cloudflared,
  legist-api,
}:
writeScript "tunnel" ''
  #!/usr/bin/env bash

  cleanup() {
      kill $(jobs -p) 2>/dev/null
  }
  trap cleanup EXIT SIGINT SIGTERM

  ${go-swag}/bin/swag init -g doc.go -d ./internal/api -o docs

  ${cloudflared}/bin/cloudflared tunnel run --token "$CLOUDFLARE_TOKEN" &
  ${legist-api}/bin/legist-api
''
