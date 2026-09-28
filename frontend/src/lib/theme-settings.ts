// 主题外观设置：圆角通过覆写 :root 的 --radius 实现，
// index.css 中 radius-sm/xl 等派生变量与 rounded-* 工具类自动跟随。

const RADIUS_KEY = 'narrat-theme-radius';
export const DEFAULT_RADIUS = '0.25rem';

export const RADIUS_PRESETS = [
  { label: '直角', value: '0rem' },
  { label: '小', value: '0.125rem' },
  { label: '默认', value: '0.25rem' },
  { label: '适中', value: '0.5rem' },
  { label: '大', value: '0.75rem' },
  { label: '圆形', value: '1rem' },
] as const;

const MIN_REM = 0;
const MAX_REM = 1;
const STEP_REM = 0.0625; // 1/16 rem

/** 把 "0.25rem" 解析为数值（rem），非法值回退默认 */
export function parseRadius(value: string): number {
  const n = Number.parseFloat(value);
  return Number.isFinite(n) ? Math.min(MAX_REM, Math.max(MIN_REM, n)) : Number.parseFloat(DEFAULT_RADIUS);
}

export function applyRadius(value: string) {
  document.documentElement.style.setProperty('--radius', value);
}

export function loadRadius(): string {
  try {
    return localStorage.getItem(RADIUS_KEY) || DEFAULT_RADIUS;
  } catch {
    return DEFAULT_RADIUS; // 隐私模式等 localStorage 不可用场景
  }
}

export function saveRadius(value: string) {
  try {
    localStorage.setItem(RADIUS_KEY, value);
  } catch {
    /* 持久化失败不影响本次生效 */
  }
}

export const radiusRange = { min: MIN_REM, max: MAX_REM, step: STEP_REM };
