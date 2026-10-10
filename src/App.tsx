import React, { useState, useEffect } from 'react';
import { BikeGeometry } from './types/bike';
import { Navbar } from './components/Navbar';
import { BikeCompare } from './components/BikeCompare';
import { BikeValidator } from './components/BikeValidator';
import { BikeEditorModal } from './components/BikeEditorModal';
import { validateBikeGeometry } from './lib/validator';
import {
  Bike,
  Plus,
  AlertTriangle,
  CheckCircle2,
  Trash2,
  Edit,
  FolderOpen,
  ArrowRight,
  Sparkles,
} from 'lucide-react';

export const App: React.FC = () => {
  const [bikes, setBikes] = useState<BikeGeometry[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [isRefreshing, setIsRefreshing] = useState<boolean>(false);
  const [activeTab, setActiveTab] = useState<'compare' | 'validator'>('compare');
  const [selectedBikeIds, setSelectedBikeIds] = useState<string[]>([]);
  const [inspectedBikeId, setInspectedBikeId] = useState<string>('');
  const [editorModalOpen, setEditorModalOpen] = useState<boolean>(false);
  const [editingBike, setEditingBike] = useState<BikeGeometry | null>(null);

  // Load bikes from backend JSON store
  const loadBikes = async () => {
    setIsRefreshing(true);
    try {
      const res = await fetch('/api/bikes');
      if (res.ok) {
        const data = await res.json();
        setBikes(data);
        if (data.length > 0) {
          // Initialize selected bikes for comparison
          setSelectedBikeIds((prev) =>
            prev.length > 0 ? prev : data.slice(0, 4).map((b: BikeGeometry) => b.id)
          );
          setInspectedBikeId((prev) =>
            prev ? prev : data[0].id
          );
        }
      }
    } catch (err) {
      console.error('Failed to load bikes:', err);
    } finally {
      setLoading(false);
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    loadBikes();
  }, []);

  // Save bike (create or update)
  const handleSaveBike = async (bikeToSave: BikeGeometry) => {
    const isExisting = bikes.some((b) => b.id === bikeToSave.id);
    const url = isExisting ? `/api/bikes/${bikeToSave.id}` : '/api/bikes';
    const method = isExisting ? 'PUT' : 'POST';

    const res = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(bikeToSave),
    });

    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.error || 'Failed to save');
    }

    const saved = await res.json();
    await loadBikes();
    setInspectedBikeId(saved.id);
    if (!selectedBikeIds.includes(saved.id)) {
      setSelectedBikeIds((prev) => [...prev, saved.id]);
    }
  };

  // Delete bike
  const handleDeleteBike = async (id: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (!confirm(`Are you sure you want to delete this bike geometry JSON file?`)) {
      return;
    }

    try {
      const res = await fetch(`/api/bikes/${id}`, { method: 'DELETE' });
      if (res.ok) {
        await loadBikes();
        setSelectedBikeIds((prev) => prev.filter((bId) => bId !== id));
        if (inspectedBikeId === id) {
          const remaining = bikes.filter((b) => b.id !== id);
          if (remaining.length > 0) setInspectedBikeId(remaining[0].id);
        }
      }
    } catch (err) {
      console.error('Failed to delete bike:', err);
    }
  };

  // Toggle bike selection for comparison
  const handleToggleSelect = (id: string) => {
    setSelectedBikeIds((prev) =>
      prev.includes(id) ? prev.filter((bId) => bId !== id) : [...prev, id]
    );
  };

  const inspectedBike = bikes.find((b) => b.id === inspectedBikeId) || bikes[0];

  return (
    <div className="min-h-screen bg-slate-950 flex flex-col text-slate-100">
      <Navbar
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        bikeCount={bikes.length}
        onAddNew={() => {
          setEditingBike(null);
          setEditorModalOpen(true);
        }}
        onRefresh={loadBikes}
        isRefreshing={isRefreshing}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
        {loading ? (
          <div className="flex flex-col items-center justify-center h-64 text-slate-400 space-y-3">
            <div className="w-8 h-8 border-2 border-amber-500 border-t-transparent rounded-full animate-spin" />
            <p className="text-sm">Reading bike geometry JSON files from disk...</p>
          </div>
        ) : bikes.length === 0 ? (
          <div className="text-center py-16 bg-slate-900 border border-slate-800 rounded-2xl p-8">
            <FolderOpen className="w-12 h-12 text-slate-500 mx-auto mb-3" />
            <h3 className="text-lg font-semibold text-white">No Bike Geometry Files Found</h3>
            <p className="text-sm text-slate-400 mt-1 max-w-md mx-auto">
              No JSON files were found in <code className="text-amber-400">data/bikes/</code>. Add your first bike geometry profile to begin.
            </p>
            <button
              onClick={() => {
                setEditingBike(null);
                setEditorModalOpen(true);
              }}
              className="mt-4 px-4 py-2 bg-amber-500 text-slate-950 rounded-lg text-sm font-bold shadow-lg"
            >
              Add First Bike
            </button>
          </div>
        ) : (
          <div className="space-y-6">
            {/* View Mode: Compare Tab */}
            {activeTab === 'compare' && (
              <BikeCompare
                allBikes={bikes}
                selectedBikeIds={selectedBikeIds}
                onToggleSelect={handleToggleSelect}
              />
            )}

            {/* View Mode: Validator Tab */}
            {activeTab === 'validator' && (
              <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
                {/* Left Drawer / Bike Selector */}
                <div className="lg:col-span-1 space-y-3">
                  <div className="flex items-center justify-between px-1">
                    <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      Local Frame Models ({bikes.length})
                    </span>
                    <button
                      onClick={() => {
                        setEditingBike(null);
                        setEditorModalOpen(true);
                      }}
                      className="text-xs text-amber-400 hover:underline flex items-center gap-1"
                    >
                      <Plus className="w-3.5 h-3.5" /> New
                    </button>
                  </div>

                  <div className="space-y-2">
                    {bikes.map((bike) => {
                      const isInspected = bike.id === inspectedBikeId;
                      const report = validateBikeGeometry(bike);

                      return (
                        <div
                          key={bike.id}
                          onClick={() => setInspectedBikeId(bike.id)}
                          className={`p-3 rounded-xl border cursor-pointer transition flex flex-col justify-between ${
                            isInspected
                              ? 'bg-slate-900 border-amber-500/50 shadow-md ring-1 ring-amber-500/30'
                              : 'bg-slate-900/60 border-slate-800/80 hover:bg-slate-900 hover:border-slate-700'
                          }`}
                        >
                          <div className="flex items-start justify-between">
                            <div>
                              <div className="font-bold text-white text-sm">
                                {bike.manufacturer} {bike.model}
                              </div>
                              <div className="text-xs text-slate-400 mt-0.5">
                                Size {bike.size} • {bike.category || 'Hardtail'}
                              </div>
                            </div>

                            <div className="flex items-center gap-1.5">
                              {report.isValid ? (
                                <span title="Geometrically Valid">
                                  <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                                </span>
                              ) : (
                                <span title="Geometry Discrepancy">
                                  <AlertTriangle className="w-4 h-4 text-rose-400" />
                                </span>
                              )}
                            </div>
                          </div>

                          <div className="flex items-center justify-between mt-3 pt-2 border-t border-slate-800/60 text-[11px] text-slate-400">
                            <div>
                              R: <span className="text-cyan-400 font-mono">{bike.reach}</span> S:{' '}
                              <span className="text-amber-400 font-mono">{bike.stack}</span>
                            </div>

                            <div className="flex items-center gap-2">
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setEditingBike(bike);
                                  setEditorModalOpen(true);
                                }}
                                className="hover:text-amber-400 transition"
                                title="Edit bike parameters"
                              >
                                <Edit className="w-3.5 h-3.5" />
                              </button>
                              <button
                                onClick={(e) => handleDeleteBike(bike.id, e)}
                                className="hover:text-rose-400 transition"
                                title="Delete bike JSON"
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>

                {/* Right Area: Deep-Dive Validator Report */}
                <div className="lg:col-span-3">
                  {inspectedBike ? (
                    <BikeValidator
                      bike={inspectedBike}
                      onEdit={(b) => {
                        setEditingBike(b);
                        setEditorModalOpen(true);
                      }}
                    />
                  ) : null}
                </div>
              </div>
            )}
          </div>
        )}
      </main>

      {/* Editor / Add Modal */}
      {editorModalOpen && (
        <BikeEditorModal
          initialBike={editingBike}
          onClose={() => {
            setEditorModalOpen(false);
            setEditingBike(null);
          }}
          onSave={handleSaveBike}
        />
      )}

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-slate-950 py-4 mt-8 text-center text-xs text-slate-500">
        <div className="max-w-7xl mx-auto px-4 flex flex-wrap items-center justify-between gap-2">
          <div>
            BikeGeo Analyst • Rigorous 2D Cartesian Frame Geometry Model • Local JSON Storage
          </div>
          <div className="font-mono text-slate-400">
            TypeScript + Vite + Express + Tailwind
          </div>
        </div>
      </footer>
    </div>
  );
};

export default App;
