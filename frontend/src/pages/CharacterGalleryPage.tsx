import { useParams } from 'react-router-dom';
import { CharacterGalleryPanel } from '../features/character/CharacterGalleryPanel';

export function CharacterGalleryPage() {
  const { id } = useParams();
  if (!id) return null;
  return (
    <div className="space-y-3">
      <h2 className="text-xl font-semibold">角色</h2>
      <CharacterGalleryPanel projectId={id} />
    </div>
  );
}
