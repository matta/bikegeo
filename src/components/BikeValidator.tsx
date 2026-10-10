import React, { useState } from 'react';
import { BikeGeometry, ValidationReport } from '../types/bike';
import { validateBikeGeometry } from '../lib/validator';
import { calculateDynamicGeometry, round, toRad } from '../lib/geometry';
import { BikeDiagram } from './BikeDiagram';
import {
  AlertTriangle,
  CheckCircle2,
  Info,
  XCircle,
  HelpCircle,
  Sliders,
  Compass,
  Layers,
  ArrowRight,
} from 'lucide-react';

interface BikeValidatorProps {
  bike: BikeGeometry;
  onEdit?: (bike: BikeGeometry) => void;
}

export const BikeValidator: React.FC<BikeValidatorProps> = ({ bike, onEdit }) => {
  const report = validateBikeGeometry(bike);

  // Interactive dynamic sag slider for this bike
  const [sagPercent, setSagPercent] = useState<number>(
    bike.sag_percentage_published || 20
  );

  const forkTravel = bike.fork_travel_mm || 120;
  const staticForkA2c = bike.fork_a2c || report.geometricA2c;
  const currentCup = bike.cup_stack ?? (bike.headset_cup_type === 'EC44' ? 12 : 0);
  const baseStructuralA2c = report.totalStructuralA2c;

  // Compute dynamic state at the interactive sag slider position
  const simulatedSagMm = (forkTravel * sagPercent) / 100;
  const simulatedForkA2c = staticForkA2c - simulatedSagMm;
  const simulatedStructuralA2c = simulatedForkA2c + currentCup;

  const dynamicPose = calculateDynamicGeometry(
    bike.reach,
    bike.stack,
    bike.hta_deg,
    bike.sta_deg,
    bike.bb_drop,
    baseStructuralA2c,
    simulatedStructuralA2c,
    bike.wheelbase || 1150,
    bike.chainstay_length || 430
  );

  return (
    <div className="space-y-6">
      {/* Header card */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-lg">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-2xl font-bold text-white">
                {bike.manufacturer} {bike.model}
              </h2>
              <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-800 text-slate-300 border border-slate-700">
                Size {bike.size}
              </span>
              <span className="px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
                {bike.category?.toUpperCase() || 'HARDTAIL'}
              </span>
            </div>
            <p className="text-sm text-slate-400 mt-1 max-w-2xl">
              {bike.notes || 'Bicycle geometry CAD validation and hardtail consistency diagnostics.'}
            </p>
          </div>

          <div className="flex items-center gap-3">
            {report.isValid ? (
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-sm font-medium">
                <CheckCircle2 className="w-4 h-4" />
                <span>Geometrically Valid</span>
              </div>
            ) : (
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm font-medium">
                <XCircle className="w-4 h-4" />
                <span>Validation Errors</span>
              </div>
            )}
            {onEdit && (
              <button
                onClick={() => onEdit(bike)}
                className="px-3 py-1.5 text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg transition"
              >
                Edit Parameters
              </button>
            )}
          </div>
        </div>

        {/* Quick spec pills */}
        <div className="grid grid-cols-2 sm:grid-cols-4 md:grid-cols-8 gap-3 mt-5 pt-4 border-t border-slate-800 text-center">
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Reach</span>
            <span className="text-base font-bold text-cyan-400">{bike.reach} <span className="text-xs font-normal text-slate-400">mm</span></span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Stack</span>
            <span className="text-base font-bold text-amber-400">{bike.stack} <span className="text-xs font-normal text-slate-400">mm</span></span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">HTA</span>
            <span className="text-base font-bold text-white">{bike.hta_deg}°</span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">STA</span>
            <span className="text-base font-bold text-white">{bike.sta_deg}°</span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">HTL</span>
            <span className="text-base font-bold text-white">{bike.htl} <span className="text-xs font-normal text-slate-400">mm</span></span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">BB Drop</span>
            <span className="text-base font-bold text-pink-400">{bike.bb_drop} <span className="text-xs font-normal text-slate-400">mm</span></span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Fork Rake</span>
            <span className="text-base font-bold text-white">{bike.fork_rake} <span className="text-xs font-normal text-slate-400">mm</span></span>
          </div>
          <div className="bg-slate-950/60 p-2.5 rounded-lg border border-slate-800">
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Wheelbase</span>
            <span className="text-base font-bold text-white">{bike.wheelbase} <span className="text-xs font-normal text-slate-400">mm</span></span>
          </div>
        </div>
      </div>

      {/* Validation diagnostic reports */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Master Stack Equation Audit */}
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-md">
          <div className="flex items-center gap-2 mb-3">
            <Compass className="w-5 h-5 text-amber-400" />
            <h3 className="text-base font-semibold text-white">Master Stack Equation Proof</h3>
          </div>
          <p className="text-xs text-slate-400 mb-4">
            Reverse-engineers the CAD baseline to verify if published Stack matches the physical front-end.
          </p>

          <div className="bg-slate-950 p-3.5 rounded-lg font-mono text-xs text-slate-300 border border-slate-800 mb-4 overflow-x-auto">
            <div className="text-slate-400">// Master Stack Equation</div>
            <div className="text-amber-300 mt-1">
              Stack = BB_Drop - Rake·cos(HTA) + (A2C + Cup + HTL)·sin(HTA)
            </div>
            <div className="text-slate-400 mt-2">// Derived Geometric A2C</div>
            <div className="text-cyan-300 mt-1">
              Geometric A2C = ({bike.stack} - {bike.bb_drop} + {bike.fork_rake}·cos({bike.hta_deg}°)) / sin({bike.hta_deg}°) - {bike.htl}
            </div>
            <div className="text-emerald-400 font-bold mt-2">
              = {report.geometricA2c} mm (Fork A2C) + {report.cupStack} mm (Cup Stack) = {report.totalStructuralA2c} mm
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex justify-between items-center text-sm py-1.5 border-b border-slate-800">
              <span className="text-slate-400">Published Fork A2C (Static):</span>
              <span className="font-semibold text-slate-200">
                {bike.fork_a2c ? `${bike.fork_a2c} mm` : 'Not specified'}
              </span>
            </div>
            <div className="flex justify-between items-center text-sm py-1.5 border-b border-slate-800">
              <span className="text-slate-400">Reverse-Engineered CAD A2C:</span>
              <span className="font-semibold text-cyan-400">{report.geometricA2c} mm</span>
            </div>
            <div className="flex justify-between items-center text-sm py-1.5 border-b border-slate-800">
              <span className="text-slate-400">Axle-to-Crown Deviation (Δ):</span>
              <span
                className={`font-semibold ${
                  report.a2cDifference !== undefined && Math.abs(report.a2cDifference) <= 6
                    ? 'text-emerald-400'
                    : report.a2cDifference !== undefined && report.a2cDifference >= 15 && report.a2cDifference <= 38
                    ? 'text-amber-400'
                    : 'text-rose-400'
                }`}
              >
                {report.a2cDifference !== undefined
                  ? `${report.a2cDifference > 0 ? `+${report.a2cDifference}` : report.a2cDifference} mm`
                  : 'N/A'}
              </span>
            </div>
            <div className="flex justify-between items-center text-sm py-1.5">
              <span className="text-slate-400">Inferred Published Mode:</span>
              <span className="font-semibold text-slate-200">
                {report.inferredPublishMode === 'static_cad'
                  ? '🟢 Static (0% Sag) Chart'
                  : report.inferredPublishMode === 'sagged_cad_15_25'
                  ? '🟡 Sagged CAD (~20% Sag)'
                  : report.inferredPublishMode === 'mismatched_cad'
                  ? '🔴 Mismatched / Inconsistent'
                  : 'Unspecified'}
              </span>
            </div>
          </div>
        </div>

        {/* Headset & Hardware Reality Check */}
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-md flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-2 mb-3">
              <Layers className="w-5 h-5 text-indigo-400" />
              <h3 className="text-base font-semibold text-white">Headset Reality & Hardtail Dynamics</h3>
            </div>
            <p className="text-xs text-slate-400 mb-4">
              Physical headset cups and hardtail pitch geometry constraints.
            </p>

            <div className="space-y-3.5">
              <div className="p-3 rounded-lg bg-slate-950 border border-slate-800">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-slate-400">Head Tube & Headset Type:</span>
                  <span className="font-semibold text-indigo-300">
                    {bike.headset_cup_type || 'Standard Tapered'}
                  </span>
                </div>
                <div className="flex items-center justify-between text-sm mt-2">
                  <span className="text-slate-400">Lower Cup Stack:</span>
                  <span className="font-semibold text-slate-200">{currentCup} mm</span>
                </div>
              </div>

              {bike.headset_cup_type === 'EC44' ? (
                <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20 text-xs text-amber-300">
                  <div className="font-semibold flex items-center gap-1.5 mb-1">
                    <Info className="w-4 h-4 text-amber-400" />
                    EC44 Lower External Cup Detected
                  </div>
                  Frames with straight 44mm head tubes running tapered steerers require an external lower cup (EC44/40). This adds <strong>12mm</strong> of physical structural height below the head tube that slackens the frame by ~0.5° compared to zero-stack (ZS) designs.
                </div>
              ) : (
                <div className="p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-400">
                  <div className="font-semibold text-slate-300 mb-1">Internal / Zero-Stack (ZS/IS)</div>
                  Tapered head tubes (e.g. ZS44/56 or IS52) have internal lower cups with minimal stack (0–2mm).
                </div>
              )}

              {/* Front Center Check */}
              <div className="p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs">
                <div className="flex justify-between items-center text-sm">
                  <span className="text-slate-400">Calculated Front Center:</span>
                  <span className="font-semibold text-slate-200">
                    {report.frontCenterCalc} mm
                  </span>
                </div>
                {bike.front_center && (
                  <div className="flex justify-between items-center text-sm mt-1">
                    <span className="text-slate-400">Published Front Center:</span>
                    <span className="font-semibold text-slate-200">{bike.front_center} mm</span>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Issues and notices list */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-md">
        <h3 className="text-base font-semibold text-white mb-3 flex items-center gap-2">
          <span>Validation Diagnostic Log</span>
          <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300">
            {report.issues.length} {report.issues.length === 1 ? 'Notice' : 'Notices'}
          </span>
        </h3>

        <div className="space-y-3">
          {report.issues.map((issue) => (
            <div
              key={issue.id}
              className={`p-3.5 rounded-lg border text-sm flex items-start gap-3 ${
                issue.severity === 'error'
                  ? 'bg-rose-500/10 border-rose-500/30 text-rose-200'
                  : issue.severity === 'warning'
                  ? 'bg-amber-500/10 border-amber-500/30 text-amber-200'
                  : issue.severity === 'success'
                  ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-200'
                  : 'bg-blue-500/10 border-blue-500/30 text-blue-200'
              }`}
            >
              {issue.severity === 'error' && <XCircle className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />}
              {issue.severity === 'warning' && <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />}
              {issue.severity === 'success' && <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />}
              {issue.severity === 'info' && <Info className="w-5 h-5 text-blue-400 shrink-0 mt-0.5" />}

              <div className="flex-1">
                <div className="font-semibold">{issue.title}</div>
                <div className="text-xs mt-0.5 opacity-90">{issue.message}</div>
                {issue.details && (
                  <div className="text-xs mt-1 opacity-75 font-mono">{issue.details}</div>
                )}
                {(issue.expected || issue.actual) && (
                  <div className="text-xs mt-1.5 flex gap-4 font-mono opacity-80">
                    {issue.expected && <span>Expected: {issue.expected}</span>}
                    {issue.actual && <span>Calculated: {issue.actual}</span>}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Interactive Hardtail Dynamic Sag Simulator */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-md">
        <div className="flex flex-wrap items-center justify-between gap-4 mb-4">
          <div className="flex items-center gap-2">
            <Sliders className="w-5 h-5 text-cyan-400" />
            <div>
              <h3 className="text-base font-semibold text-white">Dynamic Hardtail Sag Simulator</h3>
              <p className="text-xs text-slate-400">
                Hardtails compress only at the front axle, pitching around the rear wheel.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <span className="text-sm font-medium text-slate-300">Sag:</span>
            <input
              type="range"
              min="0"
              max="35"
              step="1"
              value={sagPercent}
              onChange={(e) => setSagPercent(Number(e.target.value))}
              className="w-36 accent-cyan-400 cursor-pointer"
            />
            <span className="text-base font-bold font-mono text-cyan-400 w-12 text-right">
              {sagPercent}%
            </span>
            <span className="text-xs text-slate-400">
              ({round(simulatedSagMm, 1)}mm travel)
            </span>
          </div>
        </div>

        {/* Dynamic metrics comparison */}
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 p-3 bg-slate-950 rounded-lg border border-slate-800 text-center mb-4">
          <div>
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Head Angle</span>
            <span className="text-base font-bold text-white">
              {dynamicPose.new_hta}°{' '}
              <span className="text-xs font-normal text-emerald-400">
                ({dynamicPose.delta_angle_deg >= 0 ? `+${dynamicPose.delta_angle_deg}°` : `${dynamicPose.delta_angle_deg}°`})
              </span>
            </span>
          </div>
          <div>
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Seat Angle</span>
            <span className="text-base font-bold text-white">
              {dynamicPose.new_sta}°{' '}
              <span className="text-xs font-normal text-emerald-400">
                ({dynamicPose.delta_angle_deg >= 0 ? `+${dynamicPose.delta_angle_deg}°` : `${dynamicPose.delta_angle_deg}°`})
              </span>
            </span>
          </div>
          <div>
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Dynamic Reach</span>
            <span className="text-base font-bold text-cyan-400">
              {dynamicPose.new_reach} mm{' '}
              <span className="text-xs font-normal text-slate-400">
                ({dynamicPose.delta_reach >= 0 ? `+${dynamicPose.delta_reach}` : dynamicPose.delta_reach})
              </span>
            </span>
          </div>
          <div>
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Dynamic Stack</span>
            <span className="text-base font-bold text-amber-400">
              {dynamicPose.new_stack} mm{' '}
              <span className="text-xs font-normal text-slate-400">
                ({dynamicPose.delta_stack >= 0 ? `+${dynamicPose.delta_stack}` : dynamicPose.delta_stack})
              </span>
            </span>
          </div>
          <div>
            <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Dynamic BB Drop</span>
            <span className="text-base font-bold text-pink-400">
              {dynamicPose.new_bb_drop} mm{' '}
              <span className="text-xs font-normal text-slate-400">
                ({dynamicPose.delta_bb_drop >= 0 ? `+${dynamicPose.delta_bb_drop}` : dynamicPose.delta_bb_drop})
              </span>
            </span>
          </div>
        </div>

        {/* 2D Frame schematic diagram */}
        <BikeDiagram
          bike={bike}
          canonical={{
            structural_a2c: simulatedStructuralA2c,
            active_reach: dynamicPose.new_reach,
            active_stack: dynamicPose.new_stack,
            active_hta: dynamicPose.new_hta,
            active_sta: dynamicPose.new_sta,
            active_bb_drop: dynamicPose.new_bb_drop,
            aett: 0,
            spare_vertical_stack: 0,
            spare_axial_steerer: 0,
            delta_angle_deg: dynamicPose.delta_angle_deg,
            delta_reach: dynamicPose.delta_reach,
            delta_stack: dynamicPose.delta_stack,
            delta_bb_drop: dynamicPose.delta_bb_drop,
            inferredPublishMode: 'Simulated Dynamic Sag',
          }}
          showComparison={sagPercent > 0}
        />
      </div>
    </div>
  );
};
