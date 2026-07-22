import { createBrowserRouter } from 'react-router-dom';
import { AppShell } from './AppShell';
import { ProjectsPage } from '../pages/ProjectsPage';
import { ProjectDetailPage } from '../pages/ProjectDetailPage';
import { ChapterEditorPage } from '../pages/ChapterEditorPage';
import { CharacterGalleryPage } from '../pages/CharacterGalleryPage';
import { ShotListPage } from '../pages/ShotListPage';
import { VideoPreviewPage } from '../pages/VideoPreviewPage';
import { SettingsPage } from '../pages/SettingsPage';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <ProjectsPage /> },
      { path: 'projects/:id', element: <ProjectDetailPage /> },
      { path: "projects/:id/chapters", element: <ChapterListPage /> },
      { path: "projects/:id/chapters/:n", element: <ChapterEditorPage /> },
      { path: 'projects/:id/characters', element: <CharacterGalleryPage /> },
      { path: 'projects/:id/shots', element: <ShotListPage /> },
      { path: 'projects/:id/preview/:chapter', element: <VideoPreviewPage /> },
      { path: 'settings', element: <SettingsPage /> },
    ],
  },
]);
