import { createApp } from './app.js';

const app = createApp();

const PORT = parseInt(process.env.PORT, 10) || 3000;
app.listen(PORT, '0.0.0.0', () => {
  console.log(`[blog] API listening on :${PORT}`);
});
