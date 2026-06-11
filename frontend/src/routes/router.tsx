import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { useSessionStore, type ModulePermission } from '../app/store/session'
import { AuthLayout } from '../layouts/AuthLayout'
import { ShellLayout } from '../layouts/ShellLayout'
import { ForbiddenPage } from '../shared/components/ForbiddenPage'
import { useCanAccess } from '../shared/permissions/permissions'
import type { ModuleKey } from './routeManifest'
import { LoginPage, OnboardingPage, RegisterPage } from '../features/auth'
import { DashboardPage } from '../features/dashboard'
import { TenderDetailPage, TendersPage } from '../features/tender'
import {
  BidEditorPage,
  BidListPage,
  BidNewPage,
  BidTemplatesPage,
  BidWizardPage,
} from '../features/bid'
import { ComplianceDetailPage, CompliancePage } from '../features/compliance'
import { ProjectDetailPage, ProjectsPage } from '../features/project'
import { CostDetailPage, CostsPage } from '../features/cost'
import {
  FilePreviewPage,
  KnowledgeDocsPage,
  KnowledgeHomePage,
  KnowledgeTagsPage,
  KnowledgeTemplatesPage,
} from '../features/knowledge'
import { TeamPage } from '../features/team'

function RequireAuth() {
  const isAuthenticated = useSessionStore((state) => state.isAuthenticated)
  const location = useLocation()
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />
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

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AuthLayout />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/onboarding" element={<OnboardingPage />} />
        </Route>
        <Route element={<RequireAuth />}>
          <Route element={<ShellLayout />}>
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route element={<RequirePermission module="dashboard" />}>
              <Route path="/dashboard" element={<DashboardPage />} />
            </Route>
            <Route element={<RequirePermission module="tender" />}>
              <Route path="/tenders" element={<TendersPage />} />
              <Route path="/tenders/:tenderId" element={<TenderDetailPage />} />
            </Route>
            <Route element={<RequirePermission module="bid" />}>
              <Route path="/bids" element={<BidListPage />} />
              <Route path="/bids/new" element={<BidNewPage />} />
              <Route path="/bids/templates" element={<BidTemplatesPage />} />
              <Route path="/bids/:bidId/wizard" element={<BidWizardPage />} />
              <Route path="/bids/:bidId/editor" element={<BidEditorPage />} />
            </Route>
            <Route element={<RequirePermission module="compliance" />}>
              <Route path="/compliance" element={<CompliancePage />} />
              <Route path="/compliance/:checkId" element={<ComplianceDetailPage />} />
            </Route>
            <Route element={<RequirePermission module="project" />}>
              <Route path="/projects" element={<ProjectsPage />} />
              <Route path="/projects/:projectId" element={<ProjectDetailPage />} />
            </Route>
            <Route element={<RequirePermission module="cost" />}>
              <Route path="/costs" element={<CostsPage />} />
              <Route path="/costs/:costProjectId" element={<CostDetailPage />} />
            </Route>
            <Route element={<RequirePermission module="knowledge" />}>
              <Route path="/knowledge" element={<KnowledgeHomePage />} />
              <Route path="/knowledge/docs" element={<KnowledgeDocsPage />} />
              <Route path="/knowledge/templates" element={<KnowledgeTemplatesPage />} />
              <Route path="/knowledge/tags" element={<KnowledgeTagsPage />} />
              <Route path="/files/:fileId/preview" element={<FilePreviewPage />} />
            </Route>
            <Route element={<RequirePermission module="team" />}>
              <Route path="/team" element={<TeamPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
