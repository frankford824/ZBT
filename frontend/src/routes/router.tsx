import { lazy, Suspense, type ReactNode } from 'react'
import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { useSessionStore, type ModulePermission } from '../app/store/session'
import { AuthLayout } from '../layouts/AuthLayout'
import { ShellLayout } from '../layouts/ShellLayout'
import { ForbiddenPage } from '../shared/components/ForbiddenPage'
import { LoadingBlock } from '../shared/components/StateBlocks'
import { useCanAccess } from '../shared/permissions/permissions'
import type { ModuleKey } from './routeManifest'

const LoginPage = lazy(() => import('../features/auth').then((mod) => ({ default: mod.LoginPage })))
const RegisterPage = lazy(() => import('../features/auth').then((mod) => ({ default: mod.RegisterPage })))
const OnboardingPage = lazy(() => import('../features/auth').then((mod) => ({ default: mod.OnboardingPage })))
const DashboardPage = lazy(() => import('../features/dashboard').then((mod) => ({ default: mod.DashboardPage })))
const TendersPage = lazy(() => import('../features/tender').then((mod) => ({ default: mod.TendersPage })))
const TenderDetailPage = lazy(() => import('../features/tender').then((mod) => ({ default: mod.TenderDetailPage })))
const BidListPage = lazy(() => import('../features/bid').then((mod) => ({ default: mod.BidListPage })))
const BidNewPage = lazy(() => import('../features/bid').then((mod) => ({ default: mod.BidNewPage })))
const BidTemplatesPage = lazy(() => import('../features/bid').then((mod) => ({ default: mod.BidTemplatesPage })))
const BidWizardPage = lazy(() => import('../features/bid').then((mod) => ({ default: mod.BidWizardPage })))
const BidEditorPage = lazy(() => import('../features/bid').then((mod) => ({ default: mod.BidEditorPage })))
const CompliancePage = lazy(() => import('../features/compliance').then((mod) => ({ default: mod.CompliancePage })))
const ComplianceDetailPage = lazy(() =>
  import('../features/compliance').then((mod) => ({ default: mod.ComplianceDetailPage })),
)
const ProjectsPage = lazy(() => import('../features/project').then((mod) => ({ default: mod.ProjectsPage })))
const ProjectDetailPage = lazy(() => import('../features/project').then((mod) => ({ default: mod.ProjectDetailPage })))
const CostsPage = lazy(() => import('../features/cost').then((mod) => ({ default: mod.CostsPage })))
const CostDetailPage = lazy(() => import('../features/cost').then((mod) => ({ default: mod.CostDetailPage })))
const KnowledgeHomePage = lazy(() => import('../features/knowledge').then((mod) => ({ default: mod.KnowledgeHomePage })))
const KnowledgeDocsPage = lazy(() => import('../features/knowledge').then((mod) => ({ default: mod.KnowledgeDocsPage })))
const KnowledgeTemplatesPage = lazy(() =>
  import('../features/knowledge').then((mod) => ({ default: mod.KnowledgeTemplatesPage })),
)
const KnowledgeTagsPage = lazy(() => import('../features/knowledge').then((mod) => ({ default: mod.KnowledgeTagsPage })))
const FilePreviewPage = lazy(() => import('../features/knowledge').then((mod) => ({ default: mod.FilePreviewPage })))
const TeamPage = lazy(() => import('../features/team').then((mod) => ({ default: mod.TeamPage })))

function RequireAuth() {
  const isAuthenticated = useSessionStore((state) => state.isAuthenticated)
  const location = useLocation()
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
  }
  return <Outlet />
}

function RequirePermission({
  module,
  required = 'read',
}: {
  module: ModuleKey
  required?: ModulePermission
}) {
  const allowed = useCanAccess(module, required)
  if (!allowed) return <ForbiddenPage />
  return <Outlet />
}

function page(element: ReactNode) {
  return <Suspense fallback={<LoadingBlock />}>{element}</Suspense>
}

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AuthLayout />}>
          <Route path="/login" element={page(<LoginPage />)} />
          <Route path="/register" element={page(<RegisterPage />)} />
          <Route path="/onboarding" element={page(<OnboardingPage />)} />
        </Route>
        <Route element={<RequireAuth />}>
          <Route element={<ShellLayout />}>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route element={<RequirePermission module="dashboard" />}>
              <Route path="/dashboard" element={page(<DashboardPage />)} />
            </Route>
            <Route element={<RequirePermission module="tender" />}>
              <Route path="/tenders" element={page(<TendersPage />)} />
              <Route path="/tenders/:tenderId" element={page(<TenderDetailPage />)} />
            </Route>
            <Route element={<RequirePermission module="bid" />}>
              <Route path="/bids" element={page(<BidListPage />)} />
              <Route path="/bids/new" element={page(<BidNewPage />)} />
              <Route path="/bids/templates" element={page(<BidTemplatesPage />)} />
              <Route path="/bids/:bidId/wizard" element={page(<BidWizardPage />)} />
              <Route path="/bids/:bidId/editor" element={page(<BidEditorPage />)} />
            </Route>
            <Route element={<RequirePermission module="compliance" />}>
              <Route path="/compliance" element={page(<CompliancePage />)} />
              <Route path="/compliance/:checkId" element={page(<ComplianceDetailPage />)} />
            </Route>
            <Route element={<RequirePermission module="project" />}>
              <Route path="/projects" element={page(<ProjectsPage />)} />
              <Route path="/projects/:projectId" element={page(<ProjectDetailPage />)} />
            </Route>
            <Route element={<RequirePermission module="cost" />}>
              <Route path="/costs" element={page(<CostsPage />)} />
              <Route path="/costs/:costProjectId" element={page(<CostDetailPage />)} />
            </Route>
            <Route element={<RequirePermission module="knowledge" />}>
              <Route path="/knowledge" element={page(<KnowledgeHomePage />)} />
              <Route path="/knowledge/docs" element={page(<KnowledgeDocsPage />)} />
              <Route path="/knowledge/templates" element={page(<KnowledgeTemplatesPage />)} />
              <Route path="/knowledge/tags" element={page(<KnowledgeTagsPage />)} />
            </Route>
            <Route path="/files/:fileId/preview" element={page(<FilePreviewPage />)} />
            <Route element={<RequirePermission module="team" />}>
              <Route path="/team" element={page(<TeamPage />)} />
            </Route>
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
