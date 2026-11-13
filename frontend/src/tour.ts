import { driver, type DriveStep } from 'driver.js';

export function startPageTour(storageKey: string, steps: DriveStep[]) {
  const instance = driver({
    showProgress: true,
    nextBtnText: 'Lanjut',
    prevBtnText: 'Kembali',
    doneBtnText: 'Selesai',
    progressText: '{{current}} dari {{total}}',
    onDestroyed: () => localStorage.setItem(storageKey, '1'),
    steps,
  });
  instance.drive();
  return instance;
}

export function shouldAutoTour(storageKey: string): boolean {
  return !localStorage.getItem(storageKey);
}

