import { Outlet } from 'react-router-dom';

export function AppShell() {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b px-6 py-3 bg-white">
        <h1 className="text-lg font-semibold">novel2av</h1>
      </header>
      <main className="flex-1 p-6">
        <Outlet />
      </main>
    </div>
  );
}
