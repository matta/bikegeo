export type HeadsetCupType = 'EC44' | 'ZS44' | 'ZS56' | 'IS42' | 'IS52' | 'Custom';
export type BikeCategory = 'hardtail' | 'full-suspension' | 'gravel' | 'rigid';

export interface BikeGeometry {
  id: string;
  manufacturer: string;
  model: string;
  size: string;
  year?: number;
  category?: BikeCategory;
  notes?: string;

  // Primary Frame Dimensions (mm / deg)
  reach: number;
  stack: number;
  hta_deg: number;
  sta_deg: number;
  htl: number; // Head Tube Length
  bb_drop: number; // BB Drop (positive = BB is below axle line)
  fork_rake: number; // Fork offset/rake (typically 44 or 51mm)
  wheelbase: number; // Wheelbase in mm

  // Optional Frame Dimensions
  chainstay_length?: number; // Chainstay length (default 430mm)
  front_center?: number; // Front Center in mm
  seat_tube_length?: number; // Seat tube length in mm
  wheel_size?: '29' | '27.5' | 'mixed_mullet'; // Wheel diameter standard

  // Published CAD & Hardware Assumptions
  fork_travel_mm?: number; // e.g. 100, 110, 120, 130, 140, 150
  fork_a2c?: number; // Published/stated Axle-to-Crown (mm)
  sag_percentage_published?: number; // e.g. 0 for static, 20 for 20% sag
  headset_cup_type?: HeadsetCupType; // e.g. EC44
  cup_stack?: number; // Lower headset cup stack (mm), e.g. 12 for EC44, 2 for ZS

  // Rider Fit Reference (optional defaults)
  seat_height?: number; // Rider saddle height from BB (mm), default 750mm
}

export type Severity = 'error' | 'warning' | 'info' | 'success';

export interface ValidationIssue {
  id: string;
  field?: keyof BikeGeometry;
  severity: Severity;
  title: string;
  message: string;
  details?: string;
  expected?: number | string;
  actual?: number | string;
  difference?: number;
}

export interface ValidationReport {
  bikeId: string;
  isValid: boolean;
  issues: ValidationIssue[];
  geometricA2c: number;
  statedA2c?: number;
  cupStack: number;
  totalStructuralA2c: number;
  a2cDifference?: number;
  inferredPublishMode: 'static_cad' | 'sagged_cad_15_25' | 'mismatched_cad' | 'unspecified';
  frontCenterCalc?: number;
  frontCenterDiff?: number;
}

export interface CanonicalConfig {
  targetA2c: number; // Canonical Axle to Crown in mm
  cupStack: number; // Canonical lower cup stack (mm), e.g. 12 for EC44 or 0/2 for internal
  seatHeight: number; // Rider saddle height in mm (for AETT and Steerer Stack)
  sagPercent: number; // 0 for static, 20 for standard 20% sag
  name?: string; // Optional preset name e.g. "SID 110mm (20% Sag)"
}

export interface DynamicGeometryResult {
  new_hta: number;
  new_sta: number;
  new_reach: number;
  new_stack: number;
  new_bb_drop: number;
  delta_angle_deg: number;
  delta_reach: number;
  delta_stack: number;
  delta_bb_drop: number;
  total_new_a2c: number;
}

export interface BikeFitAnalysis {
  structural_a2c: number;
  active_reach: number;
  active_stack: number;
  active_hta: number;
  active_sta: number;
  active_bb_drop: number;
  aett: number;
  spare_vertical_stack: number;
  spare_axial_steerer: number;
  delta_angle_deg: number;
  delta_reach: number;
  delta_stack: number;
  delta_bb_drop: number;
  inferredPublishMode: string;
}

export interface CanonicalComparisonRow {
  bike: BikeGeometry;
  validation: ValidationReport;
  baseline: {
    reach: number;
    stack: number;
    hta_deg: number;
    sta_deg: number;
    bb_drop: number;
    a2c: number;
    aett: number;
    spare_vertical_stack: number;
    spare_axial_steerer: number;
  };
  canonical: BikeFitAnalysis;
}
