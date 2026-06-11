import type { ModuleKey } from '../../routes/routeManifest'
import { useSessionStore, type ModulePermission } from '../../app/store/session'

export function permissionAllows(level: ModulePermission | undefined, required: ModulePermission) {
  if (required === 'none') return true
  if (!level || level === 'none') return false
  if (required === 'read') return level === 'read' || level === 'full'
  return level === 'full'
}

export function useCanAccess(module: ModuleKey, required: ModulePermission = 'read') {
  const permissions = useSessionStore((state) => state.permissions)
  return permissionAllows(permissions[module], required)
}
