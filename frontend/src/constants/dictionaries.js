export const ROLE_MAP = {
  admin: { label: '管理员', type: 'danger' },
  asset_manager: { label: '资产管理员', type: 'warning' },
  approver: { label: '审批人', type: 'success' },
  applicant: { label: '申请人', type: 'info' }
}

export const USER_STATUS_MAP = {
  active: { label: '启用', type: 'success' },
  disabled: { label: '禁用', type: 'info' }
}

export const AUTH_SOURCE_MAP = {
  local: { label: '本地', type: 'primary' },
  ad: { label: 'AD', type: 'success' }
}

export const ASSET_STATUS_MAP = {
  pending: { label: '待上线', type: 'warning' },
  in_use: { label: '使用中', type: 'success' },
  maintenance: { label: '维护中', type: 'warning' },
  retired: { label: '已退役', type: 'info' },
  decommissioned: { label: '已下线', type: 'info' }
}

export const TICKET_STATUS_MAP = {
  draft: { label: '草稿', type: 'info' },
  pending_approval: { label: '审批中', type: 'warning' },
  approved: { label: '已审批', type: 'success' },
  rejected: { label: '已驳回', type: 'danger' },
  in_progress: { label: '执行中', type: 'primary' },
  pending_acceptance: { label: '待验收', type: 'warning' },
  closed: { label: '已关闭', type: 'success' },
  cancelled: { label: '已取消', type: 'info' }
}

export const TICKET_TYPE_MAP = {
  asset_register: { label: '资产登记', type: 'primary' },
  asset_change: { label: '资产变更', type: 'warning' },
  asset_retire: { label: '资产下线/报废', type: 'danger' },
  maintenance: { label: '权限/维护申请', type: 'success' }
}

export const PRIORITY_MAP = {
  low: { label: '低', type: 'info' },
  normal: { label: '普通', type: 'primary' },
  high: { label: '高', type: 'warning' },
  urgent: { label: '紧急', type: 'danger' }
}

export function dictOptions(map) {
  return Object.entries(map).map(([value, item]) => ({ value, label: item.label }))
}

export function dictItem(map, value) {
  return map[value] || { label: value || '-', type: 'info' }
}
