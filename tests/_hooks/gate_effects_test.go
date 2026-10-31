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
		"curl --data payload --next https://api.example.com/items",
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
		"wget --post-data=x --reject-regex 'https://remote.example/.*' http://127.0.0.1/",
		"wget --post-data=x --accept-regex 'https://remote.example/.*' http://127.0.0.1/",
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

func TestGateStopsCloudPublishAndRemoteWrites(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"aws s3 rm s3://bucket/key",
		"aws s3 cp dist/ s3://bucket/ --recursive",
		"aws s3 sync ./site s3://bucket",
		"aws --profile prod ec2 terminate-instances --instance-ids i-1",
		"aws lambda update-function-code --function-name f --zip-file fileb://f.zip",
		"aws ecs update-service --cluster c --service s --force-new-deployment",
		"gcloud run deploy api --image registry.example.com/p/i",
		"gcloud --project p app deploy",
		"gcloud compute instances delete vm-1 --zone us-central1-a",
		"az webapp deploy --name app --src-path app.zip",
		"az group delete --name rg",
		"gh workflow run deploy.yml",
		"gh secret set API_KEY",
		"gh variable set ENV --body prod",
		"gh release upload v1.0.0 dist.tar.gz",
		"gh repo delete o/r",
		"gh run cancel 123",
		"kubectl patch deployment api -p '{}'",
		"kubectl rollout restart deployment/api",
		"kubectl --context prod scale deployment api --replicas=0",
		"docker buildx build --push -t registry.example.com/app .",
		"docker image push registry.example.com/app",
		"twine upload dist/*",
		"uv publish",
		"poetry publish --build",
		"gem push app.gem",
		"rsync -avz ./dist/ deploy@host.example.com:/srv/app/",
		"rsync -e ssh -a dist/ host.example.com:/srv/",
		"scp build.tar.gz deploy@host.example.com:/srv/",
		"scp -i key.pem -P 2222 app.zip host.example.com:~/",
		"psql -h db.example.com -c \"DELETE FROM users WHERE id = 1\"",
		"psql \"$DATABASE_URL\" -c 'drop table x'",
		"mysql -h db.example.com -e \"UPDATE t SET a=1\"",
		"netlify deploy --prod",
		"heroku config:set KEY=v -a app",
		"aws s3 mv s3://bucket/key ./key",
		"aws route53 change-resource-record-sets --hosted-zone-id Z123 --change-batch file://change.json",
		"gcloud storage cp ./artifact gs://bucket/artifact",
		"gsutil rm gs://bucket/old",
		"az storage blob upload-batch --account-name acct --destination container --source ./dist",
		"az account management-group create --name new-group",
		"docker buildx build --push=true -t registry.example.com/app .",
		"docker buildx build --output type=registry -t registry.example.com/app .",
		"scp artifact p:/tmp/artifact",
		"psql -h db.example.com -c BEGIN -c 'DELETE FROM users' -c COMMIT",
		"psql -hdb.example.com -c 'DELETE FROM users'",
		"psql 'host=db.example.com dbname=app' -c 'DELETE FROM users'",
		"heroku create review-app",
		"heroku run --app prod 'rake db:migrate'",
		"netlify env:set API_TOKEN value",
		"gcloud storage buckets create gs://new-bucket",
		"gcloud logging logs delete app-log --quiet",
		"psql -h db.example.com -v host=localhost -c 'DELETE FROM users'",
		"psql -h db.example.com -c \"SELECT 1 -- '\" -c \"DELETE FROM users -- '\"",
		"heroku pg:backups:delete b001 --app prod",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-cloud", command)
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

func TestGateIgnoresCloudReadsAndLocalCopies(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"aws s3 ls s3://bucket",
		"aws s3 cp s3://bucket/key ./key",
		"aws ec2 describe-instances",
		"aws sts get-caller-identity",
		"aws logs tail /aws/lambda/f",
		"gcloud config set project p",
		"gcloud run services list",
		"gcloud auth login",
		"az account set --subscription x",
		"az group list",
		"gh workflow list",
		"gh secret list",
		"gh run view 123",
		"gh release view v1",
		"kubectl get pods",
		"kubectl rollout status deployment/api",
		"docker buildx build -t app .",
		"docker image ls",
		"rsync -av host.example.com:/var/log/app/ ./logs/",
		"rsync -av ./a/ ./b/",
		"scp host.example.com:/etc/hosts ./hosts",
		"psql -c 'select 1'",
		"psql -h localhost -c 'DELETE FROM x'",
		"mysql -e 'SELECT * FROM t'",
		"heroku logs --tail -a app",
		"netlify status",
		"uv run pytest",
		"poetry install",
		"gcloud compute instances describe update --zone us-central1-a",
		"psql -h db.example.com -c \"SELECT * FROM audit_log WHERE action = 'delete'\"",
		"az account set --subscription prod",
		"gcloud storage cp gs://bucket/artifact ./artifact",
		"netlify env:list",
		"heroku apps:info -a app",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-cloud-read", command)
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence", rc, stdout, stderr)
			}
		})
	}
}
