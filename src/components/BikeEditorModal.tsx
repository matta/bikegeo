import React, { useState } from 'react';
import { BikeGeometry, HeadsetCupType, BikeCategory } from '../types/bike';
import { validateBikeGeometry } from '../lib/validator';
import { calculateGeometricA2c, round } from '../lib/geometry';
import { X, Save, AlertTriangle, CheckCircle, Code2, FormInput } from 'lucide-react';

interface BikeEditorModalProps {
  initialBike?: BikeGeometry | null;
  onClose: () => void;
  onSave: (bike: BikeGeometry) => Promise<void>;
}

const DEFAULT_BIKE: BikeGeometry = {
  id: '',
  manufacturer: '',
  model: '',
  size: 'M',
  year: 2024,
  category: 'hardtail',
  notes: '',
  reach: 450,
  stack: 630,
  hta_deg: 66.0,
  sta_deg: 75.0,
  htl: 110,
  bb_drop: 60,
  fork_rake: 44,
  wheelbase: 1180,
  chainstay_length: 430,
  fork_travel_mm: 130,
  fork_a2c: 541,
  sag_percentage_published: 0,
  headset_cup_type: 'ZS44',
  cup_stack: 2,
  seat_height: 750,
};

export const BikeEditorModal: React.FC<BikeEditorModalProps> = ({
  initialBike,
  onClose,
  onSave,
}) => {
  const [formData, setFormData] = useState<BikeGeometry>(
    initialBike ? { ...initialBike } : { ...DEFAULT_BIKE }
  );
  const [isRawJson, setIsRawJson] = useState<boolean>(false);
  const [rawJsonText, setRawJsonText] = useState<string>(
    JSON.stringify(initialBike || DEFAULT_BIKE, null, 2)
  );
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState<boolean>(false);

  // Live validation
  const validation = validateBikeGeometry(formData);

  const handleFieldChange = (key: keyof BikeGeometry, value: any) => {
    setFormData((prev) => {
      const next = { ...prev, [key]: value };
      // Auto-update cup_stack if headset cup type changed to EC44
      if (key === 'headset_cup_type' && value === 'EC44' && (!prev.cup_stack || prev.cup_stack === 0)) {
        next.cup_stack = 12.0;
      }
      return next;
    });
  };

  const handleJsonChange = (text: string) => {
    setRawJsonText(text);
    try {
      const parsed = JSON.parse(text);
      setFormData(parsed);
      setJsonError(null);
    } catch (err: any) {
      setJsonError(err.message);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSaving(true);
    try {
      // Auto generate ID if empty
      const bikeToSave = { ...formData };
      if (!bikeToSave.id) {
        bikeToSave.id = `${bikeToSave.manufacturer}-${bikeToSave.model}-${bikeToSave.size}`
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, '-')
          .replace(/(^-|-$)/g, '');
      }
      await onSave(bikeToSave);
      onClose();
    } catch (err: any) {
      alert(`Save error: ${err.message}`);
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm overflow-y-auto">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-3xl max-h-[90vh] flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-slate-800">
          <div>
            <h3 className="text-lg font-bold text-white">
              {initialBike ? `Edit: ${initialBike.manufacturer} ${initialBike.model}` : 'Add New Bike Geometry'}
            </h3>
            <p className="text-xs text-slate-400">
              Persists directly as a local JSON file in <code className="text-amber-400 font-mono">data/bikes/*.json</code>
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => {
                if (!isRawJson) {
                  setRawJsonText(JSON.stringify(formData, null, 2));
                }
                setIsRawJson(!isRawJson);
              }}
              className="px-2.5 py-1 text-xs rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 flex items-center gap-1.5 transition"
            >
              {isRawJson ? <FormInput className="w-3.5 h-3.5" /> : <Code2 className="w-3.5 h-3.5" />}
              <span>{isRawJson ? 'Form View' : 'Raw JSON'}</span>
            </button>

            <button
              onClick={onClose}
              className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Live Validation Pill Bar */}
        <div className="bg-slate-950 px-5 py-2.5 border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 text-xs">
          <div className="flex items-center gap-2">
            <span className="text-slate-400">Live Mathematical Proof:</span>
            <span className="font-mono text-cyan-400">
              Geometric A2C = {validation.geometricA2c} mm
            </span>
            {formData.fork_a2c && (
              <span className="text-slate-400 font-mono">
                (Δ {round(formData.fork_a2c - validation.geometricA2c, 1)}mm vs stated)
              </span>
            )}
          </div>

          <div>
            {validation.isValid ? (
              <span className="text-emerald-400 font-medium flex items-center gap-1">
                <CheckCircle className="w-3.5 h-3.5" /> Internally Coherent
              </span>
            ) : (
              <span className="text-rose-400 font-medium flex items-center gap-1">
                <AlertTriangle className="w-3.5 h-3.5" /> {validation.issues.length} issue(s) detected
              </span>
            )}
          </div>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-5 space-y-5">
          {isRawJson ? (
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-2">
                Raw JSON Document
              </label>
              <textarea
                value={rawJsonText}
                onChange={(e) => handleJsonChange(e.target.value)}
                rows={20}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-200 focus:outline-none focus:border-amber-500"
              />
              {jsonError && (
                <div className="text-rose-400 text-xs mt-1 font-mono">{jsonError}</div>
              )}
            </div>
          ) : (
            <div className="space-y-5">
              {/* Identity */}
              <div>
                <h4 className="text-xs font-semibold uppercase tracking-wider text-amber-400 mb-2.5">
                  Frame Identity
                </h4>
                <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Manufacturer *</label>
                    <input
                      type="text"
                      required
                      value={formData.manufacturer}
                      onChange={(e) => handleFieldChange('manufacturer', e.target.value)}
                      placeholder="e.g. Santa Cruz"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Model *</label>
                    <input
                      type="text"
                      required
                      value={formData.model}
                      onChange={(e) => handleFieldChange('model', e.target.value)}
                      placeholder="e.g. Chameleon 29"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Size *</label>
                    <input
                      type="text"
                      required
                      value={formData.size}
                      onChange={(e) => handleFieldChange('size', e.target.value)}
                      placeholder="e.g. M, L, XL"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Category</label>
                    <select
                      value={formData.category || 'hardtail'}
                      onChange={(e) => handleFieldChange('category', e.target.value as BikeCategory)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white"
                    >
                      <option value="hardtail">Hardtail</option>
                      <option value="full-suspension">Full Suspension</option>
                      <option value="gravel">Gravel</option>
                      <option value="rigid">Rigid</option>
                    </select>
                  </div>
                </div>
              </div>

              {/* Core Published Frame Geometry */}
              <div>
                <h4 className="text-xs font-semibold uppercase tracking-wider text-cyan-400 mb-2.5">
                  Published CAD Frame Dimensions
                </h4>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Reach (mm) *</label>
                    <input
                      type="number"
                      step="0.5"
                      required
                      value={formData.reach}
                      onChange={(e) => handleFieldChange('reach', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Stack (mm) *</label>
                    <input
                      type="number"
                      step="0.5"
                      required
                      value={formData.stack}
                      onChange={(e) => handleFieldChange('stack', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">HTA (°) *</label>
                    <input
                      type="number"
                      step="0.1"
                      required
                      value={formData.hta_deg}
                      onChange={(e) => handleFieldChange('hta_deg', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">STA (°) *</label>
                    <input
                      type="number"
                      step="0.1"
                      required
                      value={formData.sta_deg}
                      onChange={(e) => handleFieldChange('sta_deg', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Head Tube Length (mm) *</label>
                    <input
                      type="number"
                      step="1"
                      required
                      value={formData.htl}
                      onChange={(e) => handleFieldChange('htl', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">BB Drop (mm) *</label>
                    <input
                      type="number"
                      step="0.5"
                      required
                      value={formData.bb_drop}
                      onChange={(e) => handleFieldChange('bb_drop', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Fork Rake / Offset (mm) *</label>
                    <input
                      type="number"
                      step="1"
                      required
                      value={formData.fork_rake}
                      onChange={(e) => handleFieldChange('fork_rake', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Wheelbase (mm) *</label>
                    <input
                      type="number"
                      step="1"
                      required
                      value={formData.wheelbase}
                      onChange={(e) => handleFieldChange('wheelbase', Number(e.target.value))}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Published Fork & Headset Hardware */}
              <div>
                <h4 className="text-xs font-semibold uppercase tracking-wider text-pink-400 mb-2.5">
                  Published Fork & Headset Specs
                </h4>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Fork Travel (mm)</label>
                    <input
                      type="number"
                      value={formData.fork_travel_mm || ''}
                      onChange={(e) => handleFieldChange('fork_travel_mm', Number(e.target.value))}
                      placeholder="e.g. 130"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Stated Fork A2C (mm)</label>
                    <input
                      type="number"
                      value={formData.fork_a2c || ''}
                      onChange={(e) => handleFieldChange('fork_a2c', Number(e.target.value))}
                      placeholder="e.g. 541"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Headset Type</label>
                    <select
                      value={formData.headset_cup_type || 'ZS44'}
                      onChange={(e) => handleFieldChange('headset_cup_type', e.target.value as HeadsetCupType)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white"
                    >
                      <option value="EC44">EC44 (Straight 44mm, +12mm Cup)</option>
                      <option value="ZS44">ZS44/ZS56 (Tapered Internal)</option>
                      <option value="IS42">IS42/IS52 (Integrated)</option>
                      <option value="Custom">Custom</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Lower Cup Stack (mm)</label>
                    <input
                      type="number"
                      step="0.5"
                      value={formData.cup_stack ?? ''}
                      onChange={(e) => handleFieldChange('cup_stack', Number(e.target.value))}
                      placeholder="e.g. 12 for EC44"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Optional Parameters */}
              <div>
                <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2.5">
                  Optional Fit / Secondary Constraints
                </h4>
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Chainstay (mm)</label>
                    <input
                      type="number"
                      value={formData.chainstay_length || ''}
                      onChange={(e) => handleFieldChange('chainstay_length', Number(e.target.value))}
                      placeholder="Default 430"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Front Center (mm)</label>
                    <input
                      type="number"
                      value={formData.front_center || ''}
                      onChange={(e) => handleFieldChange('front_center', Number(e.target.value))}
                      placeholder="e.g. 792"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-slate-300 mb-1">Rider Saddle Height (mm)</label>
                    <input
                      type="number"
                      value={formData.seat_height || ''}
                      onChange={(e) => handleFieldChange('seat_height', Number(e.target.value))}
                      placeholder="Default 750"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Notes */}
              <div>
                <label className="block text-xs text-slate-300 mb-1">Notes & Specifications</label>
                <textarea
                  rows={2}
                  value={formData.notes || ''}
                  onChange={(e) => handleFieldChange('notes', e.target.value)}
                  placeholder="e.g. Published with 130mm static fork. Straight 44mm head tube."
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-xs text-white"
                />
              </div>
            </div>
          )}

          {/* Footer actions */}
          <div className="pt-4 border-t border-slate-800 flex justify-between items-center">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition"
            >
              Cancel
            </button>

            <button
              type="submit"
              disabled={isSaving}
              className="px-5 py-2 rounded-lg bg-amber-500 hover:bg-amber-400 text-slate-950 text-xs font-bold transition flex items-center gap-2 shadow-lg disabled:opacity-50"
            >
              <Save className="w-4 h-4" />
              <span>{isSaving ? 'Saving to Disk...' : 'Save Bike JSON'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
