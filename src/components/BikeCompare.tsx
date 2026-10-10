import React, { useState } from 'react';
import { BikeGeometry, CanonicalConfig } from '../types/bike';
import { COMMON_FORK_PRESETS, canonicalizeBikes, calculateSaggedA2c } from '../lib/canonicalizer';
import { BikeDiagram } from './BikeDiagram';
import {
  SlidersHorizontal,
  Scale,
  GitCompare,
  CheckCircle,
  AlertCircle,
  HelpCircle,
  ArrowUpDown,
  Filter,
} from 'lucide-react';

interface BikeCompareProps {
  allBikes: BikeGeometry[];
  selectedBikeIds: string[];
  onToggleSelect: (id: string) => void;
}

export const BikeCompare: React.FC<BikeCompareProps> = ({
  allBikes,
  selectedBikeIds,
  onToggleSelect,
}) => {
  // Canonical configuration state
  const [selectedPresetId, setSelectedPresetId] = useState<string>('fox34_130');
  const [targetStaticA2c, setTargetStaticA2c] = useState<number>(541);
  const [travelMm, setTravelMm] = useState<number>(130);
  const [sagPercent, setSagPercent] = useState<number>(20);
  const [cupMode, setCupMode] = useState<'native' | 'fixed_ec44' | 'fixed_zero'>('native');
  const [riderSaddleHeight, setRiderSaddleHeight] = useState<number>(760);
  const [referenceBikeId, setReferenceBikeId] = useState<string>(
    selectedBikeIds[0] || allBikes[0]?.id || ''
  );
  const [diagramBikeId, setDiagramBikeId] = useState<string>(
    selectedBikeIds[0] || allBikes[0]?.id || ''
  );

  // Handle preset change
  const handlePresetChange = (presetId: string) => {
    setSelectedPresetId(presetId);
    const preset = COMMON_FORK_PRESETS.find((p) => p.id === presetId);
    if (preset) {
      setTargetStaticA2c(preset.staticA2c);
      setTravelMm(preset.travelMm);
      setSagPercent(preset.standardSagPercent);
    }
  };

  // Compute effective canonical A2C
  const effectiveA2c = calculateSaggedA2c(targetStaticA2c, travelMm, sagPercent);

  // Selected bikes objects
  const selectedBikes = allBikes.filter((b) => selectedBikeIds.includes(b.id));

  // Run canonicalization
  const comparisonRows = selectedBikes.map((bike) => {
    let cupStack = bike.cup_stack ?? (bike.headset_cup_type === 'EC44' ? 12 : 0);
    if (cupMode === 'fixed_ec44') cupStack = 12;
    if (cupMode === 'fixed_zero') cupStack = 0;

    const config: CanonicalConfig = {
      targetA2c: effectiveA2c,
      cupStack,
      seatHeight: riderSaddleHeight,
      sagPercent,
    };

    const [row] = canonicalizeBikes([bike], config);
    return row;
  });

  const referenceRow = comparisonRows.find((r) => r.bike.id === referenceBikeId);
  const activeDiagramRow = comparisonRows.find((r) => r.bike.id === diagramBikeId) || comparisonRows[0];

  return (
    <div className="space-y-6">
      {/* Control Panel: Benchmark Definition */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex flex-wrap items-center justify-between gap-4 mb-4 pb-4 border-b border-slate-800">
          <div className="flex items-center gap-2.5">
            <Scale className="w-5 h-5 text-amber-400" />
            <div>
              <h2 className="text-lg font-bold text-white">
                Apples-to-Apples Canonicalization Benchmark
              </h2>
              <p className="text-xs text-slate-400">
                Pitches all frames to an identical fork axle-to-crown, sag, and headset cup configuration.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-400">Comparing:</span>
            <span className="px-2.5 py-1 rounded bg-amber-500/10 border border-amber-500/20 text-amber-300 font-bold text-xs">
              {selectedBikes.length} of {allBikes.length} Bikes
            </span>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {/* Preset Selector */}
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              Benchmark Fork Preset
            </label>
            <select
              value={selectedPresetId}
              onChange={(e) => handlePresetChange(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-amber-500"
            >
              {COMMON_FORK_PRESETS.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} (static {p.staticA2c}mm)
                </option>
              ))}
              <option value="custom">Custom Fork Specifications...</option>
            </select>
          </div>

          {/* Fork Static A2C & Travel */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-slate-300">
                Fork Static A2C / Travel
              </label>
              <span className="text-xs text-slate-400 font-mono">
                {targetStaticA2c}mm / {travelMm}mm
              </span>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <input
                type="number"
                value={targetStaticA2c}
                onChange={(e) => {
                  setTargetStaticA2c(Number(e.target.value));
                  setSelectedPresetId('custom');
                }}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-200"
                placeholder="A2C mm"
              />
              <input
                type="number"
                value={travelMm}
                onChange={(e) => {
                  setTravelMm(Number(e.target.value));
                  setSelectedPresetId('custom');
                }}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-200"
                placeholder="Travel mm"
              />
            </div>
          </div>

          {/* Sag Percentage */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-slate-300">Suspension Sag</label>
              <span className="text-xs font-bold text-cyan-400 font-mono">
                {sagPercent}% ({effectiveA2c}mm A2C)
              </span>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="range"
                min="0"
                max="35"
                step="5"
                value={sagPercent}
                onChange={(e) => setSagPercent(Number(e.target.value))}
                className="w-full accent-amber-500 cursor-pointer"
              />
              <button
                type="button"
                onClick={() => setSagPercent(sagPercent === 0 ? 20 : 0)}
                className="px-2 py-1 text-[11px] rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 whitespace-nowrap"
              >
                {sagPercent === 0 ? 'Set Sag 20%' : 'Set Static'}
              </button>
            </div>
          </div>

          {/* Rider Saddle Height */}
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs font-medium text-slate-300">
                Rider Saddle Height (BB to Saddle)
              </label>
              <span className="text-xs text-emerald-400 font-mono">
                {riderSaddleHeight} mm
              </span>
            </div>
            <input
              type="number"
              min="500"
              max="950"
              step="5"
              value={riderSaddleHeight}
              onChange={(e) => setRiderSaddleHeight(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-slate-200"
            />
          </div>
        </div>

        {/* Headset cup normalization toggle */}
        <div className="flex flex-wrap items-center justify-between gap-4 mt-4 pt-4 border-t border-slate-800 text-xs">
          <div className="flex items-center gap-2">
            <span className="text-slate-400">Headset Cup Policy:</span>
            <div className="inline-flex rounded-lg p-0.5 bg-slate-950 border border-slate-800">
              <button
                type="button"
                onClick={() => setCupMode('native')}
                className={`px-2.5 py-1 rounded text-xs transition ${
                  cupMode === 'native'
                    ? 'bg-amber-500 text-slate-950 font-semibold'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                Frame Native (EC44 = 12mm, ZS = 0/2mm)
              </button>
              <button
                type="button"
                onClick={() => setCupMode('fixed_ec44')}
                className={`px-2.5 py-1 rounded text-xs transition ${
                  cupMode === 'fixed_ec44'
                    ? 'bg-amber-500 text-slate-950 font-semibold'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                Force EC44 (+12mm) on All
              </button>
              <button
                type="button"
                onClick={() => setCupMode('fixed_zero')}
                className={`px-2.5 py-1 rounded text-xs transition ${
                  cupMode === 'fixed_zero'
                    ? 'bg-amber-500 text-slate-950 font-semibold'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                Zero-Stack (0mm) on All
              </button>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span className="text-slate-400">Reference Baseline:</span>
            <select
              value={referenceBikeId}
              onChange={(e) => setReferenceBikeId(e.target.value)}
              className="bg-slate-950 border border-slate-800 rounded px-2 py-1 text-xs text-slate-200"
            >
              {selectedBikes.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.manufacturer} {b.model} ({b.size})
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Bike Quick Toggle Selector */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-3.5 shadow-md">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold text-slate-300 uppercase tracking-wider">
            Select Bikes to Include in Comparison
          </span>
          <div className="space-x-2">
            <button
              onClick={() => allBikes.forEach((b) => !selectedBikeIds.includes(b.id) && onToggleSelect(b.id))}
              className="text-[11px] text-amber-400 hover:underline"
            >
              Select All
            </button>
            <span className="text-slate-600">|</span>
            <button
              onClick={() => {
                allBikes.forEach((b) => {
                  if (selectedBikeIds.includes(b.id) && selectedBikeIds.length > 1) {
                    onToggleSelect(b.id);
                  }
                });
              }}
              className="text-[11px] text-slate-400 hover:underline"
            >
              Clear
            </button>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {allBikes.map((bike) => {
            const isSelected = selectedBikeIds.includes(bike.id);
            return (
              <button
                key={bike.id}
                onClick={() => onToggleSelect(bike.id)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition flex items-center gap-2 border ${
                  isSelected
                    ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 shadow-sm'
                    : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800 hover:text-slate-200'
                }`}
              >
                <span
                  className={`w-2 h-2 rounded-full ${
                    isSelected ? 'bg-amber-400' : 'bg-slate-600'
                  }`}
                />
                <span>
                  {bike.manufacturer} {bike.model} ({bike.size})
                </span>
                {bike.headset_cup_type === 'EC44' && (
                  <span className="text-[10px] bg-slate-800 px-1 py-0.5 rounded text-amber-400">
                    EC44
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* Side-by-Side Comparison Matrix */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
        <div className="p-4 border-b border-slate-800 flex justify-between items-center">
          <div>
            <h3 className="font-bold text-white text-base">
              Canonicalized Geometry Comparison Matrix
            </h3>
            <p className="text-xs text-slate-400">
              Values normalized to Benchmark A2C ({effectiveA2c}mm) and Saddle Height ({riderSaddleHeight}mm).
            </p>
          </div>
          <div className="text-xs font-mono text-cyan-400">
            Delta (Δ) relative to Reference Bike
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="bg-slate-950/80 text-slate-400 border-b border-slate-800">
                <th className="p-3 font-semibold uppercase tracking-wider w-48 sticky left-0 bg-slate-950 z-10">
                  Metric
                </th>
                {comparisonRows.map(({ bike }) => (
                  <th
                    key={bike.id}
                    className={`p-3 font-semibold min-w-[210px] border-l border-slate-800/60 ${
                      bike.id === referenceBikeId ? 'bg-amber-500/5' : ''
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-white text-sm font-bold">
                        {bike.manufacturer} {bike.model}
                      </span>
                      {bike.id === referenceBikeId && (
                        <span className="text-[10px] bg-amber-500/20 text-amber-300 px-1.5 py-0.5 rounded font-mono">
                          REF
                        </span>
                      )}
                    </div>
                    <div className="text-slate-400 text-[11px] font-normal mt-0.5">
                      Size {bike.size} • {bike.headset_cup_type || 'ZS'} Headset
                    </div>
                  </th>
                ))}
              </tr>
            </thead>

            <tbody className="divide-y divide-slate-800/60">
              {/* CAD Sag Inference Row */}
              <tr className="bg-slate-950/40">
                <td className="p-2.5 font-medium text-slate-300 sticky left-0 bg-slate-900 z-10">
                  Published CAD Mode
                </td>
                {comparisonRows.map(({ bike, validation }) => (
                  <td key={bike.id} className="p-2.5 border-l border-slate-800/60 font-mono">
                    <span
                      className={`px-2 py-0.5 rounded text-[11px] inline-block ${
                        validation.inferredPublishMode === 'static_cad'
                          ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                          : validation.inferredPublishMode === 'sagged_cad_15_25'
                          ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                          : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                      }`}
                    >
                      {validation.inferredPublishMode === 'static_cad'
                        ? 'Static (0% Sag)'
                        : validation.inferredPublishMode === 'sagged_cad_15_25'
                        ? 'Sagged (~20% Sag)'
                        : 'Mismatched'}
                    </span>
                  </td>
                ))}
              </tr>

              {/* Canonical Reach */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-cyan-300 sticky left-0 bg-slate-900 z-10">
                  <div>Canonical Reach</div>
                  <div className="text-[10px] text-slate-500 font-normal">Horizontal BB to Head Tube</div>
                </td>
                {comparisonRows.map(({ bike, baseline, canonical }) => {
                  const refCanon = referenceRow?.canonical.active_reach ?? canonical.active_reach;
                  const deltaVsRef = Math.round((canonical.active_reach - refCanon) * 10) / 10;
                  const deltaVsBase = Math.round((canonical.active_reach - baseline.reach) * 10) / 10;

                  return (
                    <td key={bike.id} className="p-2.5 border-l border-slate-800/60">
                      <div className="flex items-baseline justify-between font-mono">
                        <span className="text-sm font-bold text-cyan-300">
                          {canonical.active_reach} mm
                        </span>
                        {bike.id !== referenceBikeId && (
                          <span
                            className={`text-xs ${
                              deltaVsRef > 0 ? 'text-cyan-400' : deltaVsRef < 0 ? 'text-pink-400' : 'text-slate-400'
                            }`}
                          >
                            {deltaVsRef > 0 ? `+${deltaVsRef}` : deltaVsRef} mm
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        Base: {baseline.reach}mm ({deltaVsBase >= 0 ? `+${deltaVsBase}` : deltaVsBase})
                      </div>
                    </td>
                  );
                })}
              </tr>

              {/* Canonical Stack */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-amber-300 sticky left-0 bg-slate-900 z-10">
                  <div>Canonical Stack</div>
                  <div className="text-[10px] text-slate-500 font-normal">Vertical BB to Head Tube</div>
                </td>
                {comparisonRows.map(({ bike, baseline, canonical }) => {
                  const refCanon = referenceRow?.canonical.active_stack ?? canonical.active_stack;
                  const deltaVsRef = Math.round((canonical.active_stack - refCanon) * 10) / 10;
                  const deltaVsBase = Math.round((canonical.active_stack - baseline.stack) * 10) / 10;

                  return (
                    <td key={bike.id} className="p-2.5 border-l border-slate-800/60">
                      <div className="flex items-baseline justify-between font-mono">
                        <span className="text-sm font-bold text-amber-300">
                          {canonical.active_stack} mm
                        </span>
                        {bike.id !== referenceBikeId && (
                          <span
                            className={`text-xs ${
                              deltaVsRef > 0 ? 'text-amber-400' : deltaVsRef < 0 ? 'text-pink-400' : 'text-slate-400'
                            }`}
                          >
                            {deltaVsRef > 0 ? `+${deltaVsRef}` : deltaVsRef} mm
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        Base: {baseline.stack}mm ({deltaVsBase >= 0 ? `+${deltaVsBase}` : deltaVsBase})
                      </div>
                    </td>
                  );
                })}
              </tr>

              {/* Canonical HTA */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-slate-200 sticky left-0 bg-slate-900 z-10">
                  <div>Canonical HTA</div>
                  <div className="text-[10px] text-slate-500 font-normal">Head Tube Angle</div>
                </td>
                {comparisonRows.map(({ bike, baseline, canonical }) => {
                  const refCanon = referenceRow?.canonical.active_hta ?? canonical.active_hta;
                  const deltaVsRef = Math.round((canonical.active_hta - refCanon) * 100) / 100;

                  return (
                    <td key={bike.id} className="p-2.5 border-l border-slate-800/60">
                      <div className="flex items-baseline justify-between font-mono">
                        <span className="text-sm font-bold text-white">
                          {canonical.active_hta}°
                        </span>
                        {bike.id !== referenceBikeId && (
                          <span
                            className={`text-xs ${
                              deltaVsRef > 0 ? 'text-emerald-400' : deltaVsRef < 0 ? 'text-pink-400' : 'text-slate-400'
                            }`}
                          >
                            {deltaVsRef > 0 ? `+${deltaVsRef}°` : `${deltaVsRef}°`}
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        Base: {baseline.hta_deg}° (Δ {canonical.delta_angle_deg >= 0 ? `+${canonical.delta_angle_deg}°` : `${canonical.delta_angle_deg}°`})
                      </div>
                    </td>
                  );
                })}
              </tr>

              {/* Canonical STA */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-slate-200 sticky left-0 bg-slate-900 z-10">
                  <div>Canonical STA</div>
                  <div className="text-[10px] text-slate-500 font-normal">Seat Tube Angle</div>
                </td>
                {comparisonRows.map(({ bike, baseline, canonical }) => {
                  const refCanon = referenceRow?.canonical.active_sta ?? canonical.active_sta;
                  const deltaVsRef = Math.round((canonical.active_sta - refCanon) * 100) / 100;

                  return (
                    <td key={bike.id} className="p-2.5 border-l border-slate-800/60">
                      <div className="flex items-baseline justify-between font-mono">
                        <span className="text-sm font-bold text-white">
                          {canonical.active_sta}°
                        </span>
                        {bike.id !== referenceBikeId && (
                          <span
                            className={`text-xs ${
                              deltaVsRef > 0 ? 'text-emerald-400' : deltaVsRef < 0 ? 'text-pink-400' : 'text-slate-400'
                            }`}
                          >
                            {deltaVsRef > 0 ? `+${deltaVsRef}°` : `${deltaVsRef}°`}
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        Base: {baseline.sta_deg}°
                      </div>
                    </td>
                  );
                })}
              </tr>

              {/* Canonical Bottom Bracket Drop */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-pink-300 sticky left-0 bg-slate-900 z-10">
                  <div>Canonical BB Drop</div>
                  <div className="text-[10px] text-slate-500 font-normal">Pitched around rear axle</div>
                </td>
                {comparisonRows.map(({ bike, baseline, canonical }) => {
                  const refCanon = referenceRow?.canonical.active_bb_drop ?? canonical.active_bb_drop;
                  const deltaVsRef = Math.round((canonical.active_bb_drop - refCanon) * 10) / 10;
                  const deltaVsBase = Math.round((canonical.active_bb_drop - baseline.bb_drop) * 10) / 10;

                  return (
                    <td key={bike.id} className="p-2.5 border-l border-slate-800/60">
                      <div className="flex items-baseline justify-between font-mono">
                        <span className="text-sm font-bold text-pink-300">
                          {canonical.active_bb_drop} mm
                        </span>
                        {bike.id !== referenceBikeId && (
                          <span
                            className={`text-xs ${
                              deltaVsRef > 0 ? 'text-pink-400' : deltaVsRef < 0 ? 'text-emerald-400' : 'text-slate-400'
                            }`}
                          >
                            {deltaVsRef > 0 ? `+${deltaVsRef}` : deltaVsRef} mm
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        Base: {baseline.bb_drop}mm ({deltaVsBase >= 0 ? `+${deltaVsBase}` : deltaVsBase})
                      </div>
                    </td>
                  );
                })}
              </tr>

              {/* Actual Effective Top Tube (AETT) */}
              <tr className="hover:bg-slate-800/30 bg-slate-950/20">
                <td className="p-2.5 font-medium text-emerald-300 sticky left-0 bg-slate-900 z-10">
                  <div>AETT @ {riderSaddleHeight}mm</div>
                  <div className="text-[10px] text-slate-500 font-normal">Horizontal Saddle to Steerer</div>
                </td>
                {comparisonRows.map(({ bike, baseline, canonical }) => {
                  const refCanon = referenceRow?.canonical.aett ?? canonical.aett;
                  const deltaVsRef = Math.round((canonical.aett - refCanon) * 10) / 10;

                  return (
                    <td key={bike.id} className="p-2.5 border-l border-slate-800/60">
                      <div className="flex items-baseline justify-between font-mono">
                        <span className="text-sm font-bold text-emerald-300">
                          {canonical.aett} mm
                        </span>
                        {bike.id !== referenceBikeId && (
                          <span
                            className={`text-xs ${
                              deltaVsRef > 0 ? 'text-emerald-400' : deltaVsRef < 0 ? 'text-pink-400' : 'text-slate-400'
                            }`}
                          >
                            {deltaVsRef > 0 ? `+${deltaVsRef}` : deltaVsRef} mm
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        Base AETT: {baseline.aett}mm
                      </div>
                    </td>
                  );
                })}
              </tr>

              {/* Steerer Stack (Vertical) */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-indigo-300 sticky left-0 bg-slate-900 z-10">
                  <div>Spare Steerer (Vertical)</div>
                  <div className="text-[10px] text-slate-500 font-normal">Head tube to saddle level</div>
                </td>
                {comparisonRows.map(({ bike, canonical }) => (
                  <td key={bike.id} className="p-2.5 border-l border-slate-800/60 font-mono">
                    <span className="text-sm font-bold text-indigo-300">
                      {canonical.spare_vertical_stack} mm
                    </span>
                  </td>
                ))}
              </tr>

              {/* Spare Steerer (Axial Along Steerer) */}
              <tr className="hover:bg-slate-800/30">
                <td className="p-2.5 font-medium text-indigo-300 sticky left-0 bg-slate-900 z-10">
                  <div>Spare Steerer (Axial)</div>
                  <div className="text-[10px] text-slate-500 font-normal">Length along steerer column</div>
                </td>
                {comparisonRows.map(({ bike, canonical }) => (
                  <td key={bike.id} className="p-2.5 border-l border-slate-800/60 font-mono">
                    <span className="text-sm font-bold text-indigo-300">
                      {canonical.spare_axial_steerer} mm
                    </span>
                  </td>
                ))}
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      {/* Frame 2D Diagram Inspector */}
      {activeDiagramRow && (
        <div>
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-sm font-semibold text-slate-200">
              Visual Overlay for {activeDiagramRow.bike.manufacturer} {activeDiagramRow.bike.model}
            </h3>
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-400">View diagram for:</span>
              <select
                value={diagramBikeId}
                onChange={(e) => setDiagramBikeId(e.target.value)}
                className="bg-slate-950 border border-slate-800 rounded px-2 py-1 text-xs text-slate-200"
              >
                {selectedBikes.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.manufacturer} {b.model} ({b.size})
                  </option>
                ))}
              </select>
            </div>
          </div>
          <BikeDiagram
            bike={activeDiagramRow.bike}
            canonical={activeDiagramRow.canonical}
            riderSaddleHeight={riderSaddleHeight}
            showComparison={true}
          />
        </div>
      )}
    </div>
  );
};
