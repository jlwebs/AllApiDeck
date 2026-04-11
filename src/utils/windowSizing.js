import {
  WindowCenter,
  WindowIsMaximised,
  WindowMaximise,
  WindowSetSize,
  WindowUnmaximise,
} from '../../wailsjs/runtime/runtime.js';

const MAIN_WINDOW_WIDTH = 760;
const MAIN_WINDOW_HEIGHT = 460;

function canControlWindow() {
  return typeof window !== 'undefined'
    && typeof window?.runtime?.WindowSetSize === 'function';
}

function sleep(ms) {
  return new Promise(resolve => window.setTimeout(resolve, ms));
}

export async function maximiseMainWindow() {
  if (!canControlWindow()) return false;
  try {
    WindowMaximise();
    return true;
  } catch {
    return false;
  }
}

export async function restoreMainWindowFromMaximised() {
  if (!canControlWindow()) return false;
  try {
    if (!WindowIsMaximised()) {
      return false;
    }
    WindowUnmaximise();
    await sleep(60);
    return true;
  } catch {
    return false;
  }
}

export async function restoreCompactMainWindow() {
  if (!canControlWindow()) return false;
  try {
    if (WindowIsMaximised()) {
      WindowUnmaximise();
      await sleep(60);
    }
    WindowSetSize(MAIN_WINDOW_WIDTH, MAIN_WINDOW_HEIGHT);
    await sleep(20);
    WindowCenter();
    return true;
  } catch {
    return false;
  }
}
