export function hasRole(user, role) {
  return user?.role === role
}

export function hasAnyRole(user, roles = []) {
  if (!roles.length) return true
  return roles.includes(user?.role)
}

export function canManageUsers(user) {
  return hasRole(user, 'admin')
}

export function canManageAssets(user) {
  return hasAnyRole(user, ['admin', 'asset_manager'])
}
