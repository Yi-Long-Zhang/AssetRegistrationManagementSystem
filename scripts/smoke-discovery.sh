# smoke test for dynamic asset discovery APIs (no nmap installed -> exercises degraded path)
set -e
cd "$(dirname "$0")/../backend"
cat > smoke-config.yaml << 'EOF'
http:
  addr: ":18080"
storage:
  database_path: ./data/smoke-test.db
  attachment_dir: ./data/smoke-attachments
  ticket_archive_dir: ./data/smoke-archives
  ticket_template_path: ../templates/ticket-it-change-template.docx
  libreoffice_bin: soffice
discovery:
  scan_timeout_sec: 10
  default_ports: "22,80,443"
  max_hosts: 128
EOF

CONFIG_FILE=smoke-config.yaml go run ./cmd/server > smoke-server.log 2>&1 &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null || true; rm -f smoke-config.yaml smoke-server.log; rm -rf data/smoke-test.db data/smoke-attachments data/smoke-archives" EXIT

# wait for readiness
for i in $(seq 1 30); do
  if curl -sf http://localhost:18080/healthz > /dev/null 2>&1; then break; fi
  sleep 1
done
echo "== healthz =="
curl -s http://localhost:18080/healthz
echo

TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123456"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
echo "== login token length: ${#TOKEN} =="

AUTH="Authorization: Bearer $TOKEN"

echo "== create rule (valid) =="
curl -s -X POST http://localhost:18080/api/v1/discovery/rules -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"办公网段","targets":"192.168.1.0/24","ports":"22,80","serviceDetect":true,"intervalMinutes":60,"enabled":true}'
echo

echo "== create rule (invalid target) expect 400 =="
curl -s -w "\nHTTP %{http_code}\n" -X POST http://localhost:18080/api/v1/discovery/rules -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"bad","targets":"999.1.1.1"}'

echo "== list rules =="
curl -s http://localhost:18080/api/v1/discovery/rules -H "$AUTH"
echo

echo "== test rule (nmap missing -> degraded report) =="
curl -s -X POST http://localhost:18080/api/v1/discovery/rules/1/test -H "$AUTH"
echo

echo "== trigger run (async, expect 202) =="
curl -s -w "\nHTTP %{http_code}\n" -X POST http://localhost:18080/api/v1/discovery/rules/1/run -H "$AUTH"

echo "== runs list =="
sleep 2
curl -s "http://localhost:18080/api/v1/discovery/runs" -H "$AUTH"
echo

echo "== run detail =="
curl -s http://localhost:18080/api/v1/discovery/runs/1 -H "$AUTH"
echo

echo "== assets list with onlineStatus filter =="
curl -s "http://localhost:18080/api/v1/assets?onlineStatus=unknown" -H "$AUTH"
echo

echo "== unauthorized (no token) expect 401 =="
curl -s -w "\nHTTP %{http_code}\n" http://localhost:18080/api/v1/discovery/rules

echo "SMOKE DONE"
