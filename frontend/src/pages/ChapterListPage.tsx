import { useParams } from 'react-router-dom';
import { ChapterListPanel } from '../features/chapter/ChapterListPanel';

export function ChapterListPage() {
  const { id } = useParams();
  if (!id) return null;
  return (
    <div className="space-y-3">
      <h2 className="text-xl font-semibold">章节</h2>
      <ChapterListPanel projectId={id} />
    </div>
  );
}
