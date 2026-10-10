import { BikeGeometry, BikeFitAnalysis, DynamicGeometryResult } from '../types/bike';

/**
 * Converts degrees to radians.
 */
export function toRad(deg: number): number {
  return (deg * Math.PI) / 180;
}

/**
 * Converts radians to degrees.
 */
export function toDeg(rad: number): number {
  return (rad * 180) / Math.PI;
}

/**
 * Calculates the Actual Effective Top Tube (AETT) length in mm.
 *
 * AETT is the purely horizontal projection from the center of the seat tube
 * (at the specified rider saddle height) forward to the steering axis.
 */
export function calculateAett(
  reach: number,
  stack: number,
  htaDeg: number,
  staDeg: number,
  seatHeight: number
): number {
  const hta = toRad(htaDeg);
  const sta = toRad(staDeg);

  // Coordinates of the seat position relative to Bottom Bracket (0,0)
  const sY = seatHeight * Math.sin(sta);
  const sX = -seatHeight * Math.cos(sta);

  // X-coordinate intersection on the steering axis at height s_y
  const iX = reach + (stack - sY) / Math.tan(hta);

  return iX - sX;
}

/**
 * Calculates the 'spare' steerer tube required to bring the handlebars
 * level with the saddle.
 *
 * Returns:
 * - verticalStack: Pure vertical distance from head tube top to saddle horizontal line (mm).
 * - axialLength: The physical length along the angled steerer tube to achieve that height (mm).
 */
export function calculateSteererStack(
  stack: number,
  staDeg: number,
  htaDeg: number,
  seatHeight: number
): { verticalStack: number; axialLength: number } {
  const sta = toRad(staDeg);
  const hta = toRad(htaDeg);

  const sY = seatHeight * Math.sin(sta);
  const verticalStack = sY - stack;
  const axialLength = verticalStack / Math.sin(hta);

  return { verticalStack, axialLength };
}

/**
 * Reverse-engineers the geometric Structural Axle-to-Crown (A2C) length
 * that the manufacturer's CAD software assumed to generate their Stack number.
 *
 * Master Stack Equation:
 * Stack = BB_drop - Rake * cos(HTA) + (Structural_A2C + HTL) * sin(HTA)
 *
 * => Structural_A2C = (Stack - BB_drop + Rake * cos(HTA)) / sin(HTA) - HTL
 *
 * Note: Structural_A2C includes bare fork A2C plus any external lower cup stack.
 */
export function calculateGeometricA2c(
  stack: number,
  bbDrop: number,
  htl: number,
  htaDeg: number,
  rake: number,
  cupStack: number = 0
): { structuralA2c: number; forkA2c: number } {
  const hta = toRad(htaDeg);

  const structuralA2c = (stack - bbDrop + rake * Math.cos(hta)) / Math.sin(hta) - htl;
  const forkA2c = structuralA2c - cupStack;

  return { structuralA2c, forkA2c };
}

/**
 * Calculates the new frame geometry when the fork A2C is changed,
 * simulating pitching of the frame around the rear axle.
 *
 * Considers:
 * - Changes to frame angles (HTA, STA)
 * - Changes to Bottom Bracket Drop (BB raises/lowers)
 * - Rotation of Reach & Stack vectors
 */
export function calculateDynamicGeometry(
  baseReach: number,
  baseStack: number,
  baseHta: number,
  baseSta: number,
  baseBbDrop: number,
  baseStructuralA2c: number,
  newStructuralA2c: number,
  wheelbase: number = 1150.0,
  chainstayLength: number = 430.0
): DynamicGeometryResult {
  const deltaA2c = newStructuralA2c - baseStructuralA2c;

  // Vertical lift at the front axle
  const baseHtaRad = toRad(baseHta);
  const deltaY = deltaA2c * Math.sin(baseHtaRad);

  // Rotation angle around the rear axle (radians and degrees)
  const sinTheta = Math.max(-1, Math.min(1, deltaY / wheelbase));
  const thetaRad = Math.asin(sinTheta);
  const thetaDeg = toDeg(thetaRad);

  const newHta = baseHta - thetaDeg;
  const newSta = baseSta - thetaDeg;

  // Rotate Reach/Stack vector around Bottom Bracket
  const newReach = baseReach * Math.cos(thetaRad) - baseStack * Math.sin(thetaRad);
  const newStack = baseReach * Math.sin(thetaRad) + baseStack * Math.cos(thetaRad);

  // Compute Bottom Bracket displacement pivoting around rear axle (0,0)
  // Horizontal distance of BB from rear axle:
  const cs = Math.max(chainstayLength, baseBbDrop + 1);
  const xBb = Math.sqrt(Math.max(0, cs * cs - baseBbDrop * baseBbDrop));

  // In rear axle coordinate system, BB initial coords are (xBb, -baseBbDrop)
  // Rotated by theta: yBb_new = xBb * sin(theta) + (-baseBbDrop) * cos(theta)
  // New BB drop is -yBb_new:
  const newBbDrop = baseBbDrop * Math.cos(thetaRad) - xBb * Math.sin(thetaRad);

  return {
    new_hta: round(newHta, 2),
    new_sta: round(newSta, 2),
    new_reach: round(newReach, 1),
    new_stack: round(newStack, 1),
    new_bb_drop: round(newBbDrop, 1),
    delta_angle_deg: round(thetaDeg, 2),
    delta_reach: round(newReach - baseReach, 1),
    delta_stack: round(newStack - baseStack, 1),
    delta_bb_drop: round(newBbDrop - baseBbDrop, 1),
    total_new_a2c: round(newStructuralA2c, 1),
  };
}

/**
 * Calculates theoretical Front-Center from frame geometry.
 * FC is the horizontal distance between Bottom Bracket and Front Axle.
 */
export function calculateFrontCenter(
  reach: number,
  htl: number,
  htaDeg: number,
  rake: number,
  structuralA2c: number
): number {
  const hta = toRad(htaDeg);
  // Along steerer axis, crown + head tube projects horizontally backwards from axle by:
  // (structuralA2c + htl) * cos(hta)
  // Plus rake perpendicular to steerer projects horizontally forward by:
  // rake * sin(hta)
  return reach + (structuralA2c + htl) * Math.cos(hta) + rake * Math.sin(hta);
}

/**
 * Master analysis pipeline:
 * Analyzes bike fit, reverse-engineers CAD baseline, applies optional canonical/target
 * fork & cup modifications, and derives all final dynamic cockpit and frame metrics.
 */
export function analyzeBikeFit(
  bike: BikeGeometry,
  targetA2c?: number,
  targetCupStack?: number,
  overrideSeatHeight?: number
): BikeFitAnalysis {
  const seatHeight = overrideSeatHeight ?? bike.seat_height ?? 750;
  const currentCup = bike.cup_stack ?? (bike.headset_cup_type === 'EC44' ? 12 : 0);

  // 1. Reverse engineer baseline CAD geometric A2C
  const { structuralA2c: baseStructuralA2c, forkA2c: baseForkA2c } = calculateGeometricA2c(
    bike.stack,
    bike.bb_drop,
    bike.htl,
    bike.hta_deg,
    bike.fork_rake,
    currentCup
  );

  let activeReach = bike.reach;
  let activeStack = bike.stack;
  let activeHta = bike.hta_deg;
  let activeSta = bike.sta_deg;
  let activeBbDrop = bike.bb_drop;
  let activeStructuralA2c = baseStructuralA2c;
  let deltaAngle = 0;
  let deltaReach = 0;
  let deltaStack = 0;
  let deltaBbDrop = 0;

  // 2. If target A2C provided, apply dynamic pitching
  if (targetA2c !== undefined) {
    const finalCup = targetCupStack !== undefined ? targetCupStack : currentCup;
    const totalNewA2c = targetA2c + finalCup;

    const dyn = calculateDynamicGeometry(
      bike.reach,
      bike.stack,
      bike.hta_deg,
      bike.sta_deg,
      bike.bb_drop,
      baseStructuralA2c,
      totalNewA2c,
      bike.wheelbase || 1150,
      bike.chainstay_length || 430
    );

    activeReach = dyn.new_reach;
    activeStack = dyn.new_stack;
    activeHta = dyn.new_hta;
    activeSta = dyn.new_sta;
    activeBbDrop = dyn.new_bb_drop;
    activeStructuralA2c = dyn.total_new_a2c;
    deltaAngle = dyn.delta_angle_deg;
    deltaReach = dyn.delta_reach;
    deltaStack = dyn.delta_stack;
    deltaBbDrop = dyn.delta_bb_drop;
  }

  // 3. Derived cockpit metrics
  const aett = calculateAett(activeReach, activeStack, activeHta, activeSta, seatHeight);
  const { verticalStack, axialLength } = calculateSteererStack(
    activeStack,
    activeSta,
    activeHta,
    seatHeight
  );

  // 4. Infer CAD publish mode
  let inferredPublishMode = 'Unspecified';
  if (bike.fork_a2c) {
    const diff = bike.fork_a2c - baseForkA2c;
    if (Math.abs(diff) <= 6) {
      inferredPublishMode = 'Static Fork (0% Sag CAD)';
    } else if (diff >= 15 && diff <= 35) {
      inferredPublishMode = `Sagged (~20% Sag CAD, ~${Math.round(diff)}mm sag)`;
    } else {
      inferredPublishMode = `Discrepancy (${Math.round(diff)}mm deviation)`;
    }
  }

  return {
    structural_a2c: round(activeStructuralA2c, 1),
    active_reach: round(activeReach, 1),
    active_stack: round(activeStack, 1),
    active_hta: round(activeHta, 2),
    active_sta: round(activeSta, 2),
    active_bb_drop: round(activeBbDrop, 1),
    aett: round(aett, 1),
    spare_vertical_stack: round(verticalStack, 1),
    spare_axial_steerer: round(axialLength, 1),
    delta_angle_deg: round(deltaAngle, 2),
    delta_reach: round(deltaReach, 1),
    delta_stack: round(deltaStack, 1),
    delta_bb_drop: round(deltaBbDrop, 1),
    inferredPublishMode,
  };
}

export function round(val: number, decimals: number = 1): number {
  const factor = Math.pow(10, decimals);
  return Math.round(val * factor) / factor;
}
