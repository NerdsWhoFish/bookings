import { buildTelemetry } from '@nerdswhofish/browser-telemetry/build';

await buildTelemetry({ entry: 'src/telemetry.ts', outfile: 'public/telemetry.js' });
