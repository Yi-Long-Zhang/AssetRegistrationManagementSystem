# real nmap scan test against loopback (requires portable nmap installed)
set -e
cd "$(dirname "$0")/../backend"
cat > scan-config.yaml << 'EOF'
http:
  addr: ":18080"
storage:
  database_path: ./data/scan-test.db
  attachment_dir: ./data/scan-attachments
  ticket_archive_dir: ./data/scan-archives
  ticket_template_path: ../templates/ticket-it-change-template.docx
  libreoffice_bin: soffice
discovery:
  scan_timeout_sec: 60
  default_ports: "22,80,135,139,443,445,3389,5040"
  max_hosts: 128
EOF

CONFIG_FILE=scan-config.yaml go run ./cmd/server > scan-server.log 2>&1 &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null || true; rm -f scan-config.yaml scan-server.log; rm -rf data/scan-test.db data/scan-attachments data/scan-archives" EXIT

for i in $(seq 1 30); do
  if curl -sf http://localhost:18080/healthz > /dev/null 2>&1; then break; fi
  sleep 1
done
echo "== healthz =="
curl -s http://localhost:18080/healthz; echo

TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123456"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
AUTH="Authorization: Bearer $TOKEN"
echo "== login ok (token ${#TOKEN} chars) =="

echo "== create rule (target 127.0.0.1) =="
curl -s -X POST http://localhost:18080/api/v1/discovery/rules -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"本机回环","targets":"127.0.0.1","ports":"22,80,135,139,443,445,3389,5040","serviceDetect":false,"intervalMinutes":60,"enabled":true}'
echo

echo "== test rule (real scan) =="
curl -s -X POST http://localhost:18080/api/v1/discovery/rules/1/test -H "$AUTH"
echo

echo "== trigger run (async) =="
curl -s -w "\nHTTP %{http_code}\n" -X POST http://localhost:18080/api/v1/discovery/rules/1/run -H "$AUTH"

echo "== wait for scan to finish =="
for i in $(seq 1 60); do
  STATUS=$(curl -s http://localhost:18080/api/v1/discovery/runs -H "$AUTH" | grep -o '"status":"[a-z]*"' | head -1)
  echo "  run status: $STATUS"
  case "$STATUS" in *running*) sleep 2 ;; *) break ;; esac
done

echo "== run detail =="
curl -s http://localhost:18080/api/v1/discovery/runs/1 -H "$AUTH"
echo

echo "== adopt new host =="
HOST_ID=$(curl -s http://localhost:18080/api/v1/discovery/runs/1 -H "$AUTH" | grep -o '"id":[0-9]*,"runId":1,"ip":"127.0.0.1"' | head -1 | grep -o '"id":[0-9]*' | cut -d: -f2)
echo "  host id: $HOST_ID"
curl -s -X POST http://localhost:18080/api/v1/discovery/runs/1/adopt -H "$AUTH" -H 'Content-Type: application/json' -d "{\"hostIds\":[$HOST_ID]}"
echo

echo "== assets after adopt =="
curl -s "http://localhost:18080/api/v1/assets" -H "$AUTH"
echo

echo "== second run (should show none/online, not new) =="
curl -s -X POST http://localhost:18080/api/v1/discovery/rules/1/run -H "$AUTH" > /dev/null
for i in $(seq 1 60); do
  STATUS=$(curl -s http://localhost:18080/api/v1/discovery/runs -H "$AUTH" | grep -o '"status":"[a-z]*"' | head -1)
  case "$STATUS" in *running*) sleep 2 ;; *) break ;; esac
done
curl -s http://localhost:18080/api/v1/discovery/runs/2 -H "$AUTH"
echo

echo "== asset history =="
ASSET_ID=$(curl -s "http://localhost:18080/api/v1/assets" -H "$AUTH" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
curl -s "http://localhost:18080/api/v1/assets/$ASSET_ID/history" -H "$AUTH"
echo

echo "SCAN TEST DONE"
