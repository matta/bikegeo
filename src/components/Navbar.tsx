import React from 'react';
import { Bike, GitCompare, Compass, Plus, RefreshCw } from 'lucide-react';

interface NavbarProps {
  activeTab: 'compare' | 'validator';
  setActiveTab: (tab: 'compare' | 'validator') => void;
  bikeCount: number;
  onAddNew: () => void;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  activeTab,
  setActiveTab,
  bikeCount,
  onAddNew,
  onRefresh,
  isRefreshing,
}) => {
  return (
    <header className="border-b border-slate-800 bg-slate-900/90 backdrop-blur sticky top-0 z-30">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        {/* Brand */}
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-amber-500 to-amber-600 flex items-center justify-center shadow-lg shadow-amber-500/20 text-slate-950 font-black text-xl">
            📐
          </div>
          <div>
            <h1 className="font-bold text-white text-base leading-tight flex items-center gap-2">
              <span>BikeGeo Analyst</span>
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/30 uppercase tracking-widest font-mono">
                Hardtail 2D Model
              </span>
            </h1>
            <p className="text-xs text-slate-400">
              Geometry Validation & Canonical Apples-to-Apples Comparisons
            </p>
          </div>
        </div>

        {/* View Tabs */}
        <div className="flex items-center gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800">
          <button
            onClick={() => setActiveTab('compare')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition ${
              activeTab === 'compare'
                ? 'bg-amber-500 text-slate-950 shadow-md'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <GitCompare className="w-4 h-4" />
            <span>Apples-to-Apples Compare</span>
          </button>

          <button
            onClick={() => setActiveTab('validator')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-2 transition ${
              activeTab === 'validator'
                ? 'bg-amber-500 text-slate-950 shadow-md'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Compass className="w-4 h-4" />
            <span>Geometry Validator & CAD Proof</span>
          </button>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-2.5">
          <button
            onClick={onRefresh}
            title="Reload JSON models from data/bikes"
            className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
          >
            <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin text-amber-400' : ''}`} />
          </button>

          <button
            onClick={onAddNew}
            className="px-3.5 py-2 rounded-lg bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 text-slate-950 font-bold text-xs flex items-center gap-1.5 shadow-lg shadow-amber-500/20 transition"
          >
            <Plus className="w-4 h-4" />
            <span>Add Bike JSON</span>
          </button>
        </div>
      </div>
    </header>
  );
};
