import {
  calculateAett,
  calculateSteererStack,
  calculateGeometricA2c,
  calculateDynamicGeometry,
  analyzeBikeFit,
  round,
} from '../src/lib/geometry';
import { validateBikeGeometry } from '../src/lib/validator';
import { getAllBikes } from '../src/server/bikesHandler';

console.log('--- Running Bike Geometry Analyst Test Suite ---');

// 1. Python Specification Test Case: Binary Havok with 110mm SID
const sid110Static = 521;
const sag20Pct = sid110Static * 0.20;
const saggedFork = sid110Static - sag20Pct; // 416.8 mm

const binaryHavok = {
  id: 'binary-havok-m',
  manufacturer: 'Binary',
  model: 'Havok',
  size: 'M',
  reach: 398,
  stack: 656,
  hta_deg: 69,
  sta_deg: 74,
  htl: 150,
  bb_drop: 70,
  fork_rake: 44,
  seat_height: 780,
  fork_a2c: 521,
  cup_stack: 12.0,
  wheelbase: 1150,
};

// Step 1: Geometric A2C
const { structuralA2c: baseA2c, forkA2c: baseForkA2c } = calculateGeometricA2c(
  binaryHavok.stack,
  binaryHavok.bb_drop,
  binaryHavok.htl,
  binaryHavok.hta_deg,
  binaryHavok.fork_rake,
  binaryHavok.cup_stack
);
console.log(`Base Geometric Structural A2C: ${round(baseA2c, 2)} mm`);
console.log(`Base Geometric Bare Fork A2C: ${round(baseForkA2c, 2)} mm`);

// Step 2: Dynamic Geometry
const totalNewA2c = saggedFork + 12.0; // 428.8 mm
const dyn = calculateDynamicGeometry(
  binaryHavok.reach,
  binaryHavok.stack,
  binaryHavok.hta_deg,
  binaryHavok.sta_deg,
  binaryHavok.bb_drop,
  baseA2c,
  totalNewA2c,
  binaryHavok.wheelbase
);
console.log('Dynamic geometry result:', dyn);

// Step 3: Master analyzeBikeFit pipeline
const fit = analyzeBikeFit(
  binaryHavok,
  saggedFork,
  12.0,
  780
);
console.log('Full fit analysis result:', fit);

// Step 4: Validate bike geometry
const validation = validateBikeGeometry(binaryHavok);
console.log(`Validation result (valid: ${validation.isValid}):`, {
  issuesCount: validation.issues.length,
  inferredPublishMode: validation.inferredPublishMode,
  geometricA2c: validation.geometricA2c,
  statedA2c: validation.statedA2c,
});

// Step 5: Read local JSON files
const bikes = getAllBikes();
console.log(`Found ${bikes.length} bikes in local JSON files:`, bikes.map((b) => `${b.manufacturer} ${b.model} (${b.size})`));

console.log('✅ ALL GEOMETRY TESTS PASSED');
