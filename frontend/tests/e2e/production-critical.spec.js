import { expect, test } from '@playwright/test'

test.describe.configure({ mode: 'serial' })

let api
let token
let importedAsset
const password = 'e2e-admin-456'
const apiBase = 'http://127.0.0.1:18080/api/v1'

test.beforeAll(async () => {
  let response = await apiFetch('auth/login', {
    method: 'POST',
    body: JSON.stringify({ username: 'admin', password: 'e2e-admin-123' })
  })
  expect(response.ok).toBeTruthy()
  let session = await response.json()
  response = await apiFetch('auth/change-password', {
    method: 'POST',
    headers: authHeader(session.token),
    body: JSON.stringify({ oldPassword: 'e2e-admin-123', newPassword: password })
  })
  expect(response.ok).toBeTruthy()
  response = await apiFetch('auth/login', {
    method: 'POST',
    body: JSON.stringify({ username: 'admin', password })
  })
  expect(response.ok).toBeTruthy()
  session = await response.json()
  token = session.token
})

test('logs in through the user interface', async ({ page, request }) => {
  api = apiClient(request)
  await loginThroughUI(page)
  await expect(page).toHaveURL(/\/assets$/)
  await expect(page.getByRole('heading', { name: '服务器资产' })).toBeVisible()
})

test('navigates away from audit logs without blanking the routed view', async ({ page }) => {
  await loginThroughUI(page)

  await page.getByRole('menuitem', { name: '操作审计' }).click()
  await expect(page).toHaveURL(/\/audit-logs$/)
  await expect(page.getByText('系统关键操作与变更审计日志', { exact: true })).toBeVisible()

  await page.getByRole('menuitem', { name: '服务器资产' }).click()
  await expect(page).toHaveURL(/\/assets$/)
  await expect(page.getByText('服务器资产', { exact: true }).last()).toBeVisible()

  await page.getByRole('menuitem', { name: '数据看板' }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
  await expect(page.getByText('资产总数', { exact: true })).toBeVisible()
})

test('imports an asset from the mapping CSV format', async ({ request }) => {
  api = apiClient(request)
  const csv = [
    '序号,IP地址,主机名/设备名称,MAC地址,厂商,资产类型,操作系统,开放端口,运行服务/应用,应用版本,资产归属/负责人,所在网段,备注',
    '1,10.0.0.20,e2e-imported,02:00:00:00:00:20,OpenAI,server,Linux,22,ssh,9.0,admin,10.0.0.0/24,e2e'
  ].join('\n')
  const response = await api.post('assets/import', {
    headers: authHeader(token),
    multipart: {
      file: { name: 'assets.csv', mimeType: 'text/csv', buffer: Buffer.from(csv) }
    }
  })
  expect(response.ok()).toBeTruthy()
  const list = await api.get('assets?pageSize=100', { headers: authHeader(token) })
  expect(list.ok()).toBeTruthy()
  importedAsset = (await list.json()).items.find((item) => item.ip === '10.0.0.20')
  expect(importedAsset?.hostname).toBe('e2e-imported')
})

test('discovers and adopts a host with the isolated nmap fixture', async ({ request }) => {
  api = apiClient(request)
  let response = await api.post('discovery/rules', {
    headers: authHeader(token),
    data: {
      name: 'e2e discovery',
      targets: '10.0.0.10',
      ports: '22',
      probePorts: '22',
      serviceDetect: true,
      intervalMinutes: 60,
      enabled: true
    }
  })
  expect(response.ok()).toBeTruthy()
  const rule = await response.json()
  response = await api.post(`discovery/rules/${rule.id}/run`, { headers: authHeader(token) })
  expect(response.status()).toBe(202)
  const run = await response.json()
  await expect.poll(async () => {
    const current = await api.get(`discovery/runs/${run.id}`, { headers: authHeader(token) })
    return (await current.json()).status
  }, { timeout: 15000 }).toBe('success')
  response = await api.get(`discovery/runs/${run.id}`, { headers: authHeader(token) })
  const completed = await response.json()
  const host = completed.hosts.find((item) => item.ip === '10.0.0.10')
  expect(host?.changeType).toBe('new')
  response = await api.post(`discovery/runs/${run.id}/adopt`, {
    headers: authHeader(token),
    data: { hostIds: [host.id] }
  })
  expect(response.ok()).toBeTruthy()
  expect((await response.json()).adopted).toBe(1)
})

test('completes approval, execution, acceptance, and PDF archival', async ({ request }) => {
  api = apiClient(request)
  let response = await api.put('workflows/asset_change', {
    headers: authHeader(token),
    data: {
      name: 'E2E change workflow',
      enabled: true,
      nodes: [{ name: 'Administrator approval', approverIds: [1] }]
    }
  })
  expect(response.ok()).toBeTruthy()
  response = await api.post('tickets', {
    headers: authHeader(token),
    data: {
      type: 'asset_change',
      title: 'E2E production change',
      priority: 'normal',
      assetIds: [importedAsset.id],
      description: 'End-to-end validation',
      changeContent: 'Validate production workflow'
    }
  })
  expect(response.status()).toBe(201)
  const ticket = await response.json()
  for (const [action, data] of [
    ['submit', {}],
    ['approve', { remark: 'approved' }],
    ['start', {}],
    ['complete', { result: 'completed' }],
    ['accept', { acceptanceResult: 'accepted' }]
  ]) {
    response = await api.post(`tickets/${ticket.id}/${action}`, {
      headers: authHeader(token),
      data
    })
    expect(response.ok(), `${action}: ${await response.text()}`).toBeTruthy()
  }
  response = await api.get(`tickets/${ticket.id}`, { headers: authHeader(token) })
  const closed = await response.json()
  expect(closed.status).toBe('closed')
  expect(closed.archiveNo).toMatch(/^ITCFG-/)
})

test('creates, verifies, and stages a complete encrypted restore', async ({ request }) => {
  api = apiClient(request)
  let response = await api.post('backups', { headers: authHeader(token) })
  expect(response.status()).toBe(201)
  const backup = await response.json()
  expect(backup.encrypted).toBe(true)
  expect(backup.sha256).toMatch(/^[a-f0-9]{64}$/)
  response = await api.post(`backups/${encodeURIComponent(backup.name)}/verify`, {
    headers: authHeader(token)
  })
  expect(response.ok()).toBeTruthy()
  response = await api.post(`backups/${encodeURIComponent(backup.name)}/restore`, {
    headers: authHeader(token)
  })
  expect(response.ok()).toBeTruthy()
  expect((await response.json()).restored).toBe(true)
})

function authHeader(value) {
  return { Authorization: `Bearer ${value}` }
}

function apiClient(request) {
  const url = (path) => `${apiBase}/${path}`
  return {
    get: (path, options) => request.get(url(path), options),
    post: (path, options) => request.post(url(path), options),
    put: (path, options) => request.put(url(path), options),
    delete: (path, options) => request.delete(url(path), options)
  }
}

async function apiFetch(path, options = {}) {
  return fetch(`${apiBase}/${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    }
  })
}

async function loginThroughUI(page) {
  await page.goto('/login')
  await page.getByPlaceholder('请输入账号').fill('admin')
  await page.getByPlaceholder('请输入密码').fill(password)
  await page.getByRole('button', { name: '登 录' }).click()
}
