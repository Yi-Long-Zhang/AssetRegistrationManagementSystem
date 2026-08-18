import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { spawn, spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')
const backendDir = resolve(frontendDir, '..', 'backend')
const workDir = resolve(backendDir, '.tmp', 'e2e')
const toolPath = resolve(workDir, 'bin', process.platform === 'win32' ? 'e2e-tool.exe' : 'e2e-tool')
const serverPath = resolve(workDir, 'bin', process.platform === 'win32' ? 'server.exe' : 'server')
const configTemplate = resolve(backendDir, 'tests', 'e2e', 'config.yaml')
const generatedConfig = resolve(workDir, 'config.yaml')
rmSync(workDir, { recursive: true, force: true })
mkdirSync(resolve(workDir, 'bin'), { recursive: true })

for (const [output, packagePath] of [
  [toolPath, './tests/e2e/tool'],
  [serverPath, './cmd/server']
]) {
  const build = spawnSync('go', ['build', '-o', output, packagePath], {
    cwd: backendDir,
    stdio: 'inherit'
  })
  if (build.status !== 0) process.exit(build.status ?? 1)
}

const template = readFileSync(configTemplate, 'utf8')
writeFileSync(generatedConfig, template.replaceAll('__E2E_TOOL__', toolPath.replaceAll('\\', '/')))

const child = spawn(serverPath, [], {
  cwd: backendDir,
  env: { ...process.env, CONFIG_FILE: generatedConfig },
  stdio: 'inherit'
})

let stopping = false
function stop() {
  if (stopping) return
  stopping = true
  child.kill()
  setTimeout(() => {
    child.kill('SIGKILL')
    process.exit(0)
  }, 1000)
}

for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
  process.on(signal, stop)
}
process.on('exit', () => child.kill())
child.on('exit', (code) => process.exit(code ?? 0))
