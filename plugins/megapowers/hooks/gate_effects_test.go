package main

import (
	"strings"
	"testing"
)

func TestGateStopsDeploysAndHTTPWrites(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"wrangler deploy",
		"wrangler deploy --env production",
		"wrangler -e production deploy",
		"wrangler publish",
		"wrangler pages deploy ./dist --project-name site",
		"wrangler pages publish ./dist",
		"wrangler versions deploy",
		"npx wrangler deploy",
		"npx -y wrangler@3 pages deploy dist",
		"pnpm exec wrangler deploy",
		"pnpm dlx wrangler deploy",
		"pnpm wrangler deploy",
		"yarn wrangler deploy",
		"bunx wrangler deploy",
		"bun x wrangler deploy",
		"./coolify-api.sh POST /api/v1/deploy?uuid=abc",
		"bash /opt/scripts/coolify-api.sh PATCH /api/v1/applications/abc/envs '{\"key\":\"A\"}'",
		"scripts/api DELETE https://api.example.com/v1/items/3",
		"./api.sh PUT https://api.example.com/v1/items/3 @body.json",
		"curl -X POST https://api.example.com/v1/items",
		"curl --request PATCH https://api.example.com/v1/items/1",
		"curl -XDELETE https://api.example.com/v1/items/1",
		"curl -sX POST https://api.example.com/v1/items",
		"curl -sS -d '{\"text\":\"hi\"}' https://hooks.example.com/services/x",
		"curl --json '{\"a\":1}' https://api.example.com",
		"curl -F file=@a.png https://upload.example.com",
		"curl -H 'Authorization: Bearer x' --data-binary @payload.json https://api.example.com/v1",
		"curl -T build.zip https://uploads.example.com/build.zip",
		"curl -X POST \"$API_URL/items\"",
		"curl -X POST -H 'Host: localhost' https://api.example.com/x",
		"curl -X POST http://localhost:3000/a https://api.example.com/b",
		"wget --method=DELETE https://api.example.com/x",
		"wget --post-data 'a=1' https://api.example.com/x",
		"http POST api.example.com/items name=x",
		"http api.example.com/items name=x",
		"http api.example.com/items count:=3",
		"xh PUT https://api.example.com/items/1 name=y",
		"https api.example.com/upload file@a.png",
		"curl --request=POST https://api.example.com/items",
		"curl --data=x https://api.example.com/items",
		"curl --json={} https://api.example.com/items",
		"curl -X POST https://api.example.com/items --next -X GET https://api.example.com/items",
		"curl -d a=1 https://api.example.com/a --next https://api.example.com/b",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-effect", command)
			if rc != 0 || stderr != "" {
				t.Fatalf("rc=%d stderr=%q", rc, stderr)
			}
			if stdout == "" {
				t.Fatalf("want a safe-effects denial, got silence")
			}
			if text := gateText(decodeGate(t, stdout)); !strings.Contains(text, "megapowers safe-effects skill") {
				t.Fatalf("gate message = %q, want safe-effects", text)
			}
		})
	}
}

func TestGateIgnoresHTTPNearMisses(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"wrangler dev",
		"wrangler deploy --dry-run",
		"wrangler tail",
		"wrangler whoami",
		"wrangler pages project list",
		"wrangler versions list",
		"npx wrangler dev",
		"pnpm exec vitest",
		"curl https://api.example.com/x",
		"curl -X GET https://api.example.com/x",
		"curl -I https://example.com",
		"curl -G -d q=x https://api.example.com/search",
		"curl -sSfL https://example.com/install.sh -o install.sh",
		"curl -X POST http://localhost:3000/api",
		"curl -X POST http://127.0.0.1:8080/x",
		"curl -X POST http://127.1.2.3:8080/x",
		"curl -d a=1 'http://[::1]:8080/'",
		"curl --json '{}' http://app.localhost:3000/x",
		"curl -X POST localhost:3000/x",
		"curl --unix-socket /var/run/docker.sock -X POST http://localhost/containers/x/start",
		"wget https://example.com/file.tar.gz",
		"wget --post-data a=1 http://localhost:8080",
		"wget --post-data=x --output-document response.txt http://127.0.0.1:8080/items",
		"wget --post-data=x --header Accept:json http://localhost:8080/items",
		"curl -X POST http://localhost:3000/a --next https://api.example.com/b",
		"http GET api.example.com/items",
		"http api.example.com/items q==x",
		"http api.example.com/items Authorization:token",
		"http POST localhost:3000/items name=x",
		"xh :3000/items name=x",
		"grep POST access.log",
		"grep POST /var/log/nginx/access.log",
		"grep -rn \"DELETE /api\" src",
		"rg PATCH /srv/app/src",
		"echo DELETE",
		"echo POST https://api.example.com",
		"printf '%s\\n' PUT /api/x",
		"./api.sh GET /api/v1/items",
		"./api.sh POST http://localhost:8000/api/x",
		"./api.sh POST /tmp/payload.json",
		"./api.sh POST",
		"git log --grep POST",
		"./call.sh POST https://api.example.com/x --dry-run",
		"sed -n '/POST/p' routes.txt",
		"gh api -X GET /repos/o/r",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-near", command)
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence", rc, stdout, stderr)
			}
		})
	}
}
