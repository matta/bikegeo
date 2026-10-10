import { BikeGeometry, ValidationIssue, ValidationReport } from '../types/bike';
import { calculateGeometricA2c, calculateFrontCenter, round } from './geometry';

/**
 * Validates a bike geometry dataset for internal consistency,
 * hardtail CAD conventions (static vs. sagged charts), headset cup requirements,
 * and mathematical coherence between Stack, Reach, A2C, HTL, and BB Drop.
 */
export function validateBikeGeometry(bike: BikeGeometry): ValidationReport {
  const issues: ValidationIssue[] = [];

  // 1. Essential Missing Variables Check
  const requiredFields: Array<{ key: keyof BikeGeometry; label: string }> = [
    { key: 'reach', label: 'Reach' },
    { key: 'stack', label: 'Stack' },
    { key: 'hta_deg', label: 'Head Tube Angle (HTA)' },
    { key: 'sta_deg', label: 'Seat Tube Angle (STA)' },
    { key: 'htl', label: 'Head Tube Length (HTL)' },
    { key: 'bb_drop', label: 'Bottom Bracket Drop' },
    { key: 'fork_rake', label: 'Fork Rake / Offset' },
    { key: 'wheelbase', label: 'Wheelbase' },
  ];

  for (const { key, label } of requiredFields) {
    const val = bike[key];
    if (val === undefined || val === null || isNaN(Number(val)) || Number(val) <= 0) {
      issues.push({
        id: `missing_${String(key)}`,
        field: key,
        severity: 'error',
        title: `Missing Essential Variable: ${label}`,
        message: `${label} is required to execute the mathematical proofs.`,
        details: 'The Master Stack Equation and dynamic frame pitching require non-zero physical values.',
      });
    }
  }

  // If critical fields are missing, return early with fatal report
  if (issues.some((i) => i.severity === 'error')) {
    return {
      bikeId: bike.id,
      isValid: false,
      issues,
      geometricA2c: 0,
      cupStack: bike.cup_stack ?? 0,
      totalStructuralA2c: 0,
      inferredPublishMode: 'unspecified',
    };
  }

  // 2. Physical Range Plausibility Checks
  if (bike.hta_deg < 55 || bike.hta_deg > 76) {
    issues.push({
      id: 'hta_range',
      field: 'hta_deg',
      severity: 'warning',
      title: 'Unusual Head Tube Angle',
      message: `HTA of ${bike.hta_deg}° is outside normal range (58° - 74°).`,
      actual: bike.hta_deg,
    });
  }

  if (bike.sta_deg < 65 || bike.sta_deg > 82) {
    issues.push({
      id: 'sta_range',
      field: 'sta_deg',
      severity: 'warning',
      title: 'Unusual Seat Tube Angle',
      message: `STA of ${bike.sta_deg}° is outside typical range (70° - 79°).`,
      actual: bike.sta_deg,
    });
  }

  if (bike.bb_drop < 15 || bike.bb_drop > 95) {
    issues.push({
      id: 'bb_drop_range',
      field: 'bb_drop',
      severity: 'warning',
      title: 'Atypical Bottom Bracket Drop',
      message: `BB drop of ${bike.bb_drop}mm is outside typical mountain bike range (30mm - 80mm).`,
      actual: bike.bb_drop,
    });
  }

  // 3. Headset Cup & Lower Stack Reality Check (EC44 check)
  const isStraight44 = bike.headset_cup_type === 'EC44';
  const effectiveCupStack = bike.cup_stack ?? (isStraight44 ? 12 : 0);

  if (isStraight44 && (!bike.cup_stack || bike.cup_stack < 10)) {
    issues.push({
      id: 'headset_ec44_missing_stack',
      field: 'cup_stack',
      severity: 'warning',
      title: 'EC44 Lower Cup Stack Notice',
      message: 'Frames with straight 44mm head tubes running tapered forks require an EC44 external lower cup.',
      details:
        'Standard EC44/40 cups add ~12mm of structural height below the head tube. If not accounted for in CAD or build specs, the bike will sit slacker and taller than stated.',
      expected: '12 mm',
      actual: `${effectiveCupStack} mm`,
    });
  }

  // 4. Reverse-Engineering Geometric Axle-to-Crown (Master Stack Equation Check)
  const { structuralA2c, forkA2c: calculatedForkA2c } = calculateGeometricA2c(
    bike.stack,
    bike.bb_drop,
    bike.htl,
    bike.hta_deg,
    bike.fork_rake,
    effectiveCupStack
  );

  let inferredMode: ValidationReport['inferredPublishMode'] = 'unspecified';
  let a2cDiff: number | undefined = undefined;

  if (bike.fork_a2c && bike.fork_a2c > 0) {
    a2cDiff = round(bike.fork_a2c - calculatedForkA2c, 1);

    // Difference analysis:
    // If diff is close to 0 (+- 6mm), published Stack matches stated static fork!
    if (Math.abs(a2cDiff) <= 6) {
      inferredMode = 'static_cad';
      issues.push({
        id: 'stack_static_confirmed',
        severity: 'success',
        title: 'Stack & Fork A2C Match (Static CAD)',
        message: `Geometric A2C (${round(calculatedForkA2c, 1)}mm) matches stated static fork A2C (${bike.fork_a2c}mm) within ${Math.abs(a2cDiff)}mm.`,
        details: 'The manufacturer published geometry measured with an unweighted (0% sag) fork.',
        expected: bike.fork_a2c,
        actual: round(calculatedForkA2c, 1),
      });
    }
    // If diff is between 15mm and 35mm, this is standard hardtail sag (15-25% of 100-140mm travel)!
    else if (a2cDiff >= 15 && a2cDiff <= 38) {
      inferredMode = 'sagged_cad_15_25';
      const sagEstPercent = bike.fork_travel_mm
        ? round((a2cDiff / bike.fork_travel_mm) * 100, 0)
        : round((a2cDiff / 120) * 100, 0);

      issues.push({
        id: 'sagged_cad_detected',
        severity: 'info',
        title: 'Hardtail Sagged CAD Convention Detected',
        message: `Published Stack corresponds to a sagged fork (geometric A2C = ${round(calculatedForkA2c, 1)}mm, ~${a2cDiff}mm compression).`,
        details: `The manufacturer published numbers with approximately ${sagEstPercent}% fork sag rather than static fork length (${bike.fork_a2c}mm). Canonicalizing will convert this to apples-to-apples.`,
        expected: `${bike.fork_a2c} mm (static)`,
        actual: `${round(calculatedForkA2c, 1)} mm (sagged CAD)`,
        difference: a2cDiff,
      });
    }
    // If diff is large (> 38mm or negative < -10mm), there is a geometry mismatch or error
    else {
      inferredMode = 'mismatched_cad';
      issues.push({
        id: 'stack_a2c_mismatch',
        field: 'stack',
        severity: 'warning',
        title: 'Geometric Inconsistency in Stack / A2C',
        message: `The geometric A2C required to produce Stack ${bike.stack}mm is ${round(calculatedForkA2c, 1)}mm, diverging from stated fork ${bike.fork_a2c}mm by ${a2cDiff}mm.`,
        details:
          'Possible causes: typo in published Stack or HTL, unlisted headset cup height, or chart calculated under different wheel diameter/sag assumptions.',
        expected: `${bike.fork_a2c} mm`,
        actual: `${round(calculatedForkA2c, 1)} mm`,
        difference: a2cDiff,
      });
    }
  } else if (bike.fork_travel_mm) {
    // Stated travel only (estimate standard 29" fork A2C: travel + 412mm)
    const estimatedStaticA2c = bike.fork_travel_mm + 412;
    const diffFromEstimated = round(estimatedStaticA2c - calculatedForkA2c, 1);

    if (Math.abs(diffFromEstimated) <= 8) {
      inferredMode = 'static_cad';
      issues.push({
        id: 'estimated_static_match',
        severity: 'info',
        title: 'Consistent with Standard Static Fork Travel',
        message: `Geometric A2C (${round(calculatedForkA2c, 1)}mm) aligns with typical ${bike.fork_travel_mm}mm 29er fork (~${estimatedStaticA2c}mm static).`,
      });
    } else if (diffFromEstimated >= 15 && diffFromEstimated <= 38) {
      inferredMode = 'sagged_cad_15_25';
      issues.push({
        id: 'estimated_sagged_match',
        severity: 'info',
        title: 'Probable Sagged Geometry Chart',
        message: `Geometric A2C (${round(calculatedForkA2c, 1)}mm) is ~${diffFromEstimated}mm shorter than a static ${bike.fork_travel_mm}mm fork, indicating sagged CAD baseline.`,
      });
    }
  }

  // 5. Front-Center & Wheelbase Consistency Check
  const calculatedFc = round(
    calculateFrontCenter(
      bike.reach,
      bike.htl,
      bike.hta_deg,
      bike.fork_rake,
      structuralA2c
    ),
    1
  );

  let fcDiff: number | undefined = undefined;

  if (bike.front_center && bike.front_center > 0) {
    fcDiff = round(Math.abs(bike.front_center - calculatedFc), 1);
    if (fcDiff > 12) {
      issues.push({
        id: 'front_center_mismatch',
        field: 'front_center',
        severity: 'warning',
        title: 'Front Center Inconsistency',
        message: `Published Front Center (${bike.front_center}mm) differs from geometrically calculated FC (${calculatedFc}mm) by ${fcDiff}mm.`,
        details: 'Check if Reach or Fork Rake was modified in published geometry.',
        expected: `${calculatedFc} mm (calculated)`,
        actual: `${bike.front_center} mm (published)`,
        difference: fcDiff,
      });
    } else {
      issues.push({
        id: 'front_center_ok',
        severity: 'success',
        title: 'Front Center Geometrically Coherent',
        message: `Published Front Center matches calculated 2D vector within ${fcDiff}mm.`,
      });
    }
  } else if (bike.wheelbase && bike.chainstay_length) {
    // Check if Wheelbase ≈ Horizontal Chainstay + Front Center
    const csHorizontal = Math.sqrt(
      Math.max(0, bike.chainstay_length * bike.chainstay_length - bike.bb_drop * bike.bb_drop)
    );
    const estimatedWb = round(csHorizontal + calculatedFc, 1);
    const wbDiff = round(Math.abs(bike.wheelbase - estimatedWb), 1);

    if (wbDiff > 15) {
      issues.push({
        id: 'wheelbase_discrepancy',
        field: 'wheelbase',
        severity: 'warning',
        title: 'Wheelbase & Chainstay Discrepancy',
        message: `Wheelbase (${bike.wheelbase}mm) does not match Rear Center (${round(csHorizontal, 1)}mm) + Front Center (${calculatedFc}mm) = ${estimatedWb}mm (Δ ${wbDiff}mm).`,
      });
    }
  }

  const hasErrors = issues.some((i) => i.severity === 'error');

  return {
    bikeId: bike.id,
    isValid: !hasErrors,
    issues,
    geometricA2c: round(calculatedForkA2c, 1),
    statedA2c: bike.fork_a2c,
    cupStack: effectiveCupStack,
    totalStructuralA2c: round(structuralA2c, 1),
    a2cDifference: a2cDiff,
    inferredPublishMode: inferredMode,
    frontCenterCalc: calculatedFc,
    frontCenterDiff: fcDiff,
  };
}
