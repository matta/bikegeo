import express from 'express';
import cors from 'cors';
import path from 'path';
import {
  getAllBikes,
  getBikeById,
  saveBike,
  deleteBike,
} from './bikesHandler';
import { validateBikeGeometry } from '../lib/validator';
import { canonicalizeBikes } from '../lib/canonicalizer';
import { analyzeBikeFit } from '../lib/geometry';
import { BikeGeometry, CanonicalConfig } from '../types/bike';

export function createApiRouter() {
  const router = express.Router();

  router.get('/bikes', (req, res) => {
    try {
      const bikes = getAllBikes();
      res.json(bikes);
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    }
  });

  router.get('/bikes/:id', (req, res) => {
    try {
      const bike = getBikeById(req.params.id);
      if (!bike) {
        return res.status(404).json({ error: 'Bike not found' });
      }
      res.json(bike);
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    }
  });

  router.post('/bikes', (req, res) => {
    try {
      const bikeData: BikeGeometry = req.body;
      const saved = saveBike(bikeData);
      res.status(201).json(saved);
    } catch (err: any) {
      res.status(400).json({ error: err.message });
    }
  });

  router.put('/bikes/:id', (req, res) => {
    try {
      const bikeData: BikeGeometry = { ...req.body, id: req.params.id };
      const saved = saveBike(bikeData);
      res.json(saved);
    } catch (err: any) {
      res.status(400).json({ error: err.message });
    }
  });

  router.delete('/bikes/:id', (req, res) => {
    try {
      const ok = deleteBike(req.params.id);
      if (!ok) {
        return res.status(404).json({ error: 'Bike not found' });
      }
      res.json({ success: true, id: req.params.id });
    } catch (err: any) {
      res.status(500).json({ error: err.message });
    }
  });

  router.post('/validate', (req, res) => {
    try {
      const bike: BikeGeometry = req.body;
      const report = validateBikeGeometry(bike);
      res.json(report);
    } catch (err: any) {
      res.status(400).json({ error: err.message });
    }
  });

  router.post('/canonicalize', (req, res) => {
    try {
      const { bikes, config }: { bikes: BikeGeometry[]; config: CanonicalConfig } = req.body;
      const result = canonicalizeBikes(bikes, config);
      res.json(result);
    } catch (err: any) {
      res.status(400).json({ error: err.message });
    }
  });

  router.post('/analyze', (req, res) => {
    try {
      const { bike, targetA2c, targetCupStack, seatHeight } = req.body;
      const result = analyzeBikeFit(bike, targetA2c, targetCupStack, seatHeight);
      res.json(result);
    } catch (err: any) {
      res.status(400).json({ error: err.message });
    }
  });

  return router;
}

export function startServer(port: number = 3001) {
  const app = express();
  app.use(cors());
  app.use(express.json());

  app.use('/api', createApiRouter());

  // Serve static assets in production
  const distDir = path.resolve(process.cwd(), 'dist');
  app.use(express.static(distDir));
  app.use((req, res, next) => {
    if (req.path.startsWith('/api')) return next();
    res.sendFile(path.join(distDir, 'index.html'), (err) => {
      if (err) res.status(404).send('Not found. Run pnpm build first.');
    });
  });

  const server = app.listen(port, () => {
    console.log(`Bicycle Geometry Analyst Server listening on http://localhost:${port}`);
  });

  return server;
}
