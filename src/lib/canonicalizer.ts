import {
  BikeGeometry,
  CanonicalConfig,
  CanonicalComparisonRow,
  ValidationReport,
} from '../types/bike';
import { analyzeBikeFit, calculateAett, calculateSteererStack, round } from './geometry';
import { validateBikeGeometry } from './validator';

export interface ForkPreset {
  id: string;
  name: string;
  travelMm: number;
  staticA2c: number;
  standardSagPercent: number;
  description: string;
}

export const COMMON_FORK_PRESETS: ForkPreset[] = [
  {
    id: 'sid_110',
    name: 'RockShox SID 110mm',
    travelMm: 110,
    staticA2c: 521,
    standardSagPercent: 20,
    description: 'Downcountry / Marathon fork (static 521mm, 20% sag = 499mm)',
  },
  {
    id: 'sid_100',
    name: 'RockShox SID 100mm',
    travelMm: 100,
    staticA2c: 506,
    standardSagPercent: 20,
    description: 'XC Race fork (static 506mm, 20% sag = 486mm)',
  },
  {
    id: 'fox34_120',
    name: 'Fox Float 34 120mm',
    travelMm: 120,
    staticA2c: 530,
    standardSagPercent: 20,
    description: 'Downcountry standard (static 530mm, 20% sag = 506mm)',
  },
  {
    id: 'fox34_130',
    name: 'Fox Float 34 130mm',
    travelMm: 130,
    staticA2c: 541,
    standardSagPercent: 20,
    description: 'Trail hardtail benchmark (static 541mm, 20% sag = 515mm)',
  },
  {
    id: 'pike_140',
    name: 'RockShox Pike 140mm',
    travelMm: 140,
    staticA2c: 551,
    standardSagPercent: 20,
    description: 'Aggressive trail fork (static 551mm, 20% sag = 523mm)',
  },
  {
    id: 'fox36_150',
    name: 'Fox 36 / Lyrik 150mm',
    travelMm: 150,
    staticA2c: 561,
    standardSagPercent: 20,
    description: 'Hardcore hardtail / Enduro (static 561mm, 20% sag = 531mm)',
  },
];

/**
 * Calculates effective A2C taking sag into consideration.
 */
export function calculateSaggedA2c(staticA2c: number, travelMm: number, sagPercent: number): number {
  const sagMm = travelMm * (sagPercent / 100);
  return round(staticA2c - sagMm, 1);
}

/**
 * Normalizes and canonicalizes a collection of bikes to an apples-to-apples configuration.
 */
export function canonicalizeBikes(
  bikes: BikeGeometry[],
  config: CanonicalConfig
): CanonicalComparisonRow[] {
  return bikes.map((bike) => {
    const validation = validateBikeGeometry(bike);

    // Compute baseline cockpit metrics at published geometry
    const seatHeight = config.seatHeight || bike.seat_height || 750;
    const baselineAett = calculateAett(
      bike.reach,
      bike.stack,
      bike.hta_deg,
      bike.sta_deg,
      seatHeight
    );
    const { verticalStack, axialLength } = calculateSteererStack(
      bike.stack,
      bike.sta_deg,
      bike.hta_deg,
      seatHeight
    );

    // Baseline stats
    const baseline = {
      reach: bike.reach,
      stack: bike.stack,
      hta_deg: bike.hta_deg,
      sta_deg: bike.sta_deg,
      bb_drop: bike.bb_drop,
      a2c: bike.fork_a2c ?? validation.geometricA2c,
      aett: round(baselineAett, 1),
      spare_vertical_stack: round(verticalStack, 1),
      spare_axial_steerer: round(axialLength, 1),
    };

    // Canonicalized stats (target fork + target cup stack)
    const canonical = analyzeBikeFit(
      bike,
      config.targetA2c,
      config.cupStack,
      seatHeight
    );

    return {
      bike,
      validation,
      baseline,
      canonical,
    };
  });
}
