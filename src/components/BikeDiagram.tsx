import React from 'react';
import { BikeGeometry, BikeFitAnalysis } from '../types/bike';
import { toRad } from '../lib/geometry';

interface BikeDiagramProps {
  bike: BikeGeometry;
  canonical?: BikeFitAnalysis;
  showComparison?: boolean;
  riderSaddleHeight?: number;
}

interface FramePoints {
  ra: [number, number]; // Rear Axle
  bb: [number, number]; // Bottom Bracket
  tht: [number, number]; // Top Head Tube
  bht: [number, number]; // Bottom Head Tube
  crown: [number, number]; // Fork Crown
  fa: [number, number]; // Front Axle
  saddle: [number, number]; // Saddle
  seatClamp: [number, number]; // Seat Tube top
}

function calculateFrameGeometryPoints(
  reach: number,
  stack: number,
  htaDeg: number,
  staDeg: number,
  htl: number,
  bbDrop: number,
  rake: number,
  a2c: number,
  cupStack: number,
  wheelbase: number,
  chainstay: number = 430,
  saddleHeight: number = 750,
  seatTubeLength: number = 440
): FramePoints {
  const hta = toRad(htaDeg);
  const sta = toRad(staDeg);

  // Rear axle is at origin
  const ra: [number, number] = [0, 0];

  // BB position:
  const cs = Math.max(chainstay, bbDrop + 1);
  const xBb = Math.sqrt(Math.max(0, cs * cs - bbDrop * bbDrop));
  const yBb = -bbDrop;
  const bb: [number, number] = [xBb, yBb];

  // Top of Head Tube:
  const xTht = xBb + reach;
  const yTht = yBb + stack;
  const tht: [number, number] = [xTht, yTht];

  // Head tube vector goes down and forward at angle HTA:
  // (Unit vector along steerer down & forward: [cos(hta), -sin(hta)])
  const xBht = xTht + htl * Math.cos(hta);
  const yBht = yTht - htl * Math.sin(hta);
  const bht: [number, number] = [xBht, yBht];

  // Lower headset cup:
  const xCrown = xBht + cupStack * Math.cos(hta);
  const yCrown = yBht - cupStack * Math.sin(hta);
  const crown: [number, number] = [xCrown, yCrown];

  // Front Axle:
  // Axle is along steerer by a2c plus rake forward perpendicular to steerer:
  // Perpendicular forward unit vector is [sin(hta), cos(hta)]
  const xFa = xCrown + a2c * Math.cos(hta) + rake * Math.sin(hta);
  const yFa = yCrown - a2c * Math.sin(hta) + rake * Math.cos(hta);
  const fa: [number, number] = [xFa, yFa];

  // Saddle position:
  const xSaddle = xBb - saddleHeight * Math.cos(sta);
  const ySaddle = yBb + saddleHeight * Math.sin(sta);
  const saddle: [number, number] = [xSaddle, ySaddle];

  // Seat clamp (top of seat tube frame):
  const stLen = Math.min(seatTubeLength, saddleHeight - 100);
  const xClamp = xBb - stLen * Math.cos(sta);
  const yClamp = yBb + stLen * Math.sin(sta);
  const seatClamp: [number, number] = [xClamp, yClamp];

  return { ra, bb, tht, bht, crown, fa, saddle, seatClamp };
}

export const BikeDiagram: React.FC<BikeDiagramProps> = ({
  bike,
  canonical,
  showComparison = true,
  riderSaddleHeight = 750,
}) => {
  const currentCup = bike.cup_stack ?? (bike.headset_cup_type === 'EC44' ? 12 : 0);
  const baseA2c = bike.fork_a2c ?? 520;

  // Base frame points
  const basePts = calculateFrameGeometryPoints(
    bike.reach,
    bike.stack,
    bike.hta_deg,
    bike.sta_deg,
    bike.htl,
    bike.bb_drop,
    bike.fork_rake,
    baseA2c,
    currentCup,
    bike.wheelbase,
    bike.chainstay_length || 430,
    riderSaddleHeight,
    bike.seat_tube_length || 440
  );

  // Canonical points if provided
  const canonPts = canonical
    ? calculateFrameGeometryPoints(
        canonical.active_reach,
        canonical.active_stack,
        canonical.active_hta,
        canonical.active_sta,
        bike.htl,
        canonical.active_bb_drop,
        bike.fork_rake,
        canonical.structural_a2c - (bike.cup_stack ?? currentCup),
        bike.cup_stack ?? currentCup,
        bike.wheelbase,
        bike.chainstay_length || 430,
        riderSaddleHeight,
        bike.seat_tube_length || 440
      )
    : null;

  // SVG coordinate transformation:
  // Real world coordinates: X: -400 to 1400 mm, Y: -400 to 900 mm.
  // We flip Y for SVG screen coordinates.
  const svgWidth = 720;
  const svgHeight = 440;
  const padX = 260;
  const padY = 240;
  const worldMinX = -200;
  const worldMaxX = 1450;
  const worldMinY = -400;
  const worldMaxY = 950;

  const scale = Math.min(
    (svgWidth - 60) / (worldMaxX - worldMinX),
    (svgHeight - 60) / (worldMaxY - worldMinY)
  );

  const toSvgX = (x: number) => 80 + (x - worldMinX) * scale;
  const toSvgY = (y: number) => svgHeight - 60 - (y - worldMinY) * scale;

  const renderFrameSvg = (pts: FramePoints, color: string, isOverlay: boolean = false) => {
    const strokeWidth = isOverlay ? 2.5 : 2;
    const opacity = isOverlay ? 1 : 0.45;
    const wheelRadius = 370 * scale; // 29" wheel radius approx 370mm

    return (
      <g stroke={color} opacity={opacity}>
        {/* Wheels */}
        <circle
          cx={toSvgX(pts.ra[0])}
          cy={toSvgY(pts.ra[1])}
          r={wheelRadius}
          fill="none"
          strokeDasharray="4 4"
          strokeWidth="1.2"
        />
        <circle
          cx={toSvgX(pts.fa[0])}
          cy={toSvgY(pts.fa[1])}
          r={wheelRadius}
          fill="none"
          strokeDasharray="4 4"
          strokeWidth="1.2"
        />

        {/* Axle dots */}
        <circle cx={toSvgX(pts.ra[0])} cy={toSvgY(pts.ra[1])} r={4} fill={color} />
        <circle cx={toSvgX(pts.fa[0])} cy={toSvgY(pts.fa[1])} r={4} fill={color} />
        <circle cx={toSvgX(pts.bb[0])} cy={toSvgY(pts.bb[1])} r={5} fill={color} />

        {/* Chainstay: RA -> BB */}
        <line
          x1={toSvgX(pts.ra[0])}
          y1={toSvgY(pts.ra[1])}
          x2={toSvgX(pts.bb[0])}
          y2={toSvgY(pts.bb[1])}
          strokeWidth={strokeWidth}
        />

        {/* Seatstay: RA -> Seat Clamp */}
        <line
          x1={toSvgX(pts.ra[0])}
          y1={toSvgY(pts.ra[1])}
          x2={toSvgX(pts.seatClamp[0])}
          y2={toSvgY(pts.seatClamp[1])}
          strokeWidth={strokeWidth}
        />

        {/* Seat Tube: BB -> Clamp -> Saddle */}
        <line
          x1={toSvgX(pts.bb[0])}
          y1={toSvgY(pts.bb[1])}
          x2={toSvgX(pts.seatClamp[0])}
          y2={toSvgY(pts.seatClamp[1])}
          strokeWidth={strokeWidth + 1}
        />
        <line
          x1={toSvgX(pts.seatClamp[0])}
          y1={toSvgY(pts.seatClamp[1])}
          x2={toSvgX(pts.saddle[0])}
          y2={toSvgY(pts.saddle[1])}
          strokeWidth={strokeWidth}
          strokeDasharray={isOverlay ? undefined : '2 2'}
        />

        {/* Saddle symbol */}
        <line
          x1={toSvgX(pts.saddle[0]) - 25}
          y1={toSvgY(pts.saddle[1])}
          x2={toSvgX(pts.saddle[0]) + 15}
          y2={toSvgY(pts.saddle[1])}
          strokeWidth={4}
          strokeLinecap="round"
        />

        {/* Down Tube: BB -> BHT */}
        <line
          x1={toSvgX(pts.bb[0])}
          y1={toSvgY(pts.bb[1])}
          x2={toSvgX(pts.bht[0])}
          y2={toSvgY(pts.bht[1])}
          strokeWidth={strokeWidth + 1}
        />

        {/* Top Tube: Seat Clamp -> THT */}
        <line
          x1={toSvgX(pts.seatClamp[0])}
          y1={toSvgY(pts.seatClamp[1])}
          x2={toSvgX(pts.tht[0])}
          y2={toSvgY(pts.tht[1])}
          strokeWidth={strokeWidth}
        />

        {/* Head Tube: BHT -> THT */}
        <line
          x1={toSvgX(pts.bht[0])}
          y1={toSvgY(pts.bht[1])}
          x2={toSvgX(pts.tht[0])}
          y2={toSvgY(pts.tht[1])}
          strokeWidth={strokeWidth + 2.5}
        />

        {/* Lower cup stack if > 0 */}
        {bike.cup_stack && bike.cup_stack > 0 ? (
          <line
            x1={toSvgX(pts.bht[0])}
            y1={toSvgY(pts.bht[1])}
            x2={toSvgX(pts.crown[0])}
            y2={toSvgY(pts.crown[1])}
            stroke="#f59e0b"
            strokeWidth={strokeWidth + 1}
          />
        ) : null}

        {/* Fork: Crown -> Front Axle */}
        <line
          x1={toSvgX(pts.crown[0])}
          y1={toSvgY(pts.crown[1])}
          x2={toSvgX(pts.fa[0])}
          y2={toSvgY(pts.fa[1])}
          strokeWidth={strokeWidth + 1.5}
        />
      </g>
    );
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-xl">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
        <div>
          <h4 className="font-semibold text-slate-100 text-sm flex items-center gap-2">
            <span>2D Cartesian Frame & Geometry Diagram</span>
            <span className="text-xs font-normal text-slate-400">
              (Rear Axle Origin (0,0) • Pitch Rotation Overlay)
            </span>
          </h4>
        </div>
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-1.5">
            <span className="w-3 h-0.5 bg-slate-400 inline-block"></span>
            <span className="text-slate-400">Published CAD Baseline</span>
          </div>
          {canonical && showComparison && (
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-0.5 bg-cyan-400 inline-block"></span>
              <span className="text-cyan-400 font-medium">Canonical Dynamic Pose</span>
            </div>
          )}
        </div>
      </div>

      <div className="relative overflow-hidden rounded-lg bg-slate-950/80 border border-slate-800/80 flex items-center justify-center">
        <svg
          viewBox={`0 0 ${svgWidth} ${svgHeight}`}
          className="w-full h-auto max-h-[380px] select-none"
        >
          {/* Grid lines */}
          <line
            x1="20"
            y1={toSvgY(0)}
            x2={svgWidth - 20}
            y2={toSvgY(0)}
            stroke="#334155"
            strokeWidth="0.8"
            strokeDasharray="3 3"
          />
          <text
            x={svgWidth - 95}
            y={toSvgY(0) - 6}
            fill="#64748b"
            fontSize="10"
            fontFamily="monospace"
          >
            Axle Plane (Y=0)
          </text>

          {/* Render published baseline in muted gray */}
          {renderFrameSvg(basePts, '#94a3b8', !canonical || !showComparison)}

          {/* Render canonicalized pose in bright cyan if active */}
          {canonical && showComparison && canonPts && (
            renderFrameSvg(canonPts, '#38bdf8', true)
          )}

          {/* Dimension annotations on active pose */}
          {(() => {
            const active = canonPts && showComparison ? canonPts : basePts;
            const rVal = canonical && showComparison ? canonical.active_reach : bike.reach;
            const sVal = canonical && showComparison ? canonical.active_stack : bike.stack;

            return (
              <g className="text-xs">
                {/* Reach line: horizontal from BB X to THT X at THT height */}
                <line
                  x1={toSvgX(active.bb[0])}
                  y1={toSvgY(active.tht[1])}
                  x2={toSvgX(active.tht[0])}
                  y2={toSvgY(active.tht[1])}
                  stroke="#38bdf8"
                  strokeWidth="1.2"
                  strokeDasharray="3 3"
                />
                <text
                  x={(toSvgX(active.bb[0]) + toSvgX(active.tht[0])) / 2}
                  y={toSvgY(active.tht[1]) - 6}
                  fill="#38bdf8"
                  fontSize="10"
                  textAnchor="middle"
                  fontFamily="monospace"
                >
                  Reach {rVal}mm
                </text>

                {/* Stack line: vertical from BB height to THT height at BB X */}
                <line
                  x1={toSvgX(active.bb[0])}
                  y1={toSvgY(active.bb[1])}
                  x2={toSvgX(active.bb[0])}
                  y2={toSvgY(active.tht[1])}
                  stroke="#fbbf24"
                  strokeWidth="1.2"
                  strokeDasharray="3 3"
                />
                <text
                  x={toSvgX(active.bb[0]) - 8}
                  y={(toSvgY(active.bb[1]) + toSvgY(active.tht[1])) / 2}
                  fill="#fbbf24"
                  fontSize="10"
                  textAnchor="end"
                  fontFamily="monospace"
                >
                  Stack {sVal}mm
                </text>

                {/* BB Drop annotation */}
                <line
                  x1={toSvgX(active.bb[0])}
                  y1={toSvgY(0)}
                  x2={toSvgX(active.bb[0])}
                  y2={toSvgY(active.bb[1])}
                  stroke="#ec4899"
                  strokeWidth="1.2"
                />
                <text
                  x={toSvgX(active.bb[0]) + 8}
                  y={(toSvgY(0) + toSvgY(active.bb[1])) / 2 + 3}
                  fill="#ec4899"
                  fontSize="9"
                  fontFamily="monospace"
                >
                  BB Drop {canonical && showComparison ? canonical.active_bb_drop : bike.bb_drop}mm
                </text>
              </g>
            );
          })()}
        </svg>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-4 mt-3 text-xs text-slate-400">
        <div>
          <span className="text-amber-400 font-medium">HTA: </span>
          {canonical && showComparison ? `${canonical.active_hta}°` : `${bike.hta_deg}°`}
          {' • '}
          <span className="text-emerald-400 font-medium">STA: </span>
          {canonical && showComparison ? `${canonical.active_sta}°` : `${bike.sta_deg}°`}
          {' • '}
          <span className="text-rose-400 font-medium">BB Drop: </span>
          {canonical && showComparison ? `${canonical.active_bb_drop}mm` : `${bike.bb_drop}mm`}
        </div>
        {canonical && showComparison && (
          <div className="font-mono text-cyan-400">
            Pitch Rotation: {canonical.delta_angle_deg >= 0 ? `+${canonical.delta_angle_deg}°` : `${canonical.delta_angle_deg}°`}
          </div>
        )}
      </div>
    </div>
  );
};
