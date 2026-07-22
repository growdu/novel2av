import { useParams } from 'react-router-dom';
import { useProject } from '../features/project/api';
import { ShotList } from '../features/shot/ShotList';
import { openProjectSocket } from '../lib/ws/projectSocket';

export function ShotListPage() {
  const { id } = useParams();
  const proj = useProject(id);
  const aspect = proj.data?.config?.aspect;

  if (!id) return null;
  return (
    <div className="space-y-3">
      <h2 className="text-xl font-semibold">分镜</h2>
      <ShotList projectId={id} aspect={aspect ?? '9:16'} />
      {/* one socket per page; it auto-disconnects on unmount */}
      <ProjectSocketPinger projectId={id} />
    </div>
  );
}

// Tiny invisible component that keeps a live socket open so WS-routed progress
// events flow into the React Query cache via window-level handlers registered
// by useProjectSocket (see lib/ws/projectSocket).
import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
function ProjectSocketPinger({ projectId }: { projectId: string }) {
  const qc = useQueryClient();
  useEffect(() => {
    return openProjectSocket(projectId, (ev) => {
      if (ev.type !== 'job.progress') return;
      // Invalidate list-level caches opportunistically.
      qc.invalidateQueries({ queryKey: ['shots', projectId] });
      qc.invalidateQueries({ queryKey: ['characters', projectId] });
      qc.invalidateQueries({ queryKey: ['chapters', projectId] });
    });
  }, [projectId, qc]);
  return null;
}
