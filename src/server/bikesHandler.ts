import fs from 'fs';
import path from 'path';
import { BikeGeometry, CanonicalConfig } from '../types/bike';
import { validateBikeGeometry } from '../lib/validator';
import { canonicalizeBikes } from '../lib/canonicalizer';
import { analyzeBikeFit } from '../lib/geometry';

export const DATA_DIR = path.resolve(process.cwd(), 'data/bikes');

export function ensureDataDir() {
  if (!fs.existsSync(DATA_DIR)) {
    fs.mkdirSync(DATA_DIR, { recursive: true });
  }
}

export function getAllBikes(): BikeGeometry[] {
  ensureDataDir();
  const files = fs.readdirSync(DATA_DIR).filter((f) => f.endsWith('.json'));
  const bikes: BikeGeometry[] = [];

  for (const file of files) {
    try {
      const content = fs.readFileSync(path.join(DATA_DIR, file), 'utf-8');
      const bike = JSON.parse(content) as BikeGeometry;
      bikes.push(bike);
    } catch (err) {
      console.error(`Error reading ${file}:`, err);
    }
  }

  // Sort by manufacturer, model, size
  return bikes.sort((a, b) =>
    `${a.manufacturer} ${a.model} ${a.size}`.localeCompare(
      `${b.manufacturer} ${b.model} ${b.size}`
    )
  );
}

export function getBikeById(id: string): BikeGeometry | null {
  ensureDataDir();
  const filePath = path.join(DATA_DIR, `${id}.json`);
  if (!fs.existsSync(filePath)) {
    return null;
  }
  const content = fs.readFileSync(filePath, 'utf-8');
  return JSON.parse(content) as BikeGeometry;
}

export function saveBike(bike: BikeGeometry): BikeGeometry {
  ensureDataDir();
  if (!bike.id) {
    const rawId = `${bike.manufacturer}-${bike.model}-${bike.size}`
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/(^-|-$)/g, '');
    bike.id = rawId || `bike-${Date.now()}`;
  }

  const filePath = path.join(DATA_DIR, `${bike.id}.json`);
  fs.writeFileSync(filePath, JSON.stringify(bike, null, 2), 'utf-8');
  return bike;
}

export function deleteBike(id: string): boolean {
  ensureDataDir();
  const filePath = path.join(DATA_DIR, `${id}.json`);
  if (fs.existsSync(filePath)) {
    fs.unlinkSync(filePath);
    return true;
  }
  return false;
}
