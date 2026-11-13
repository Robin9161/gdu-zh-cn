import { formatSize } from '../format';

export function formatOverviewSize(bytes: number, useSIPrefix: boolean): string {
  const gigabyte = useSIPrefix ? 1_000_000_000 : 2 ** 30;
  const terabyte = gigabyte * (useSIPrefix ? 1000 : 1024);
  if (bytes >= gigabyte && bytes < terabyte) {
    return `${(bytes / gigabyte).toFixed(2)} ${useSIPrefix ? 'GB' : 'GiB'}`;
  }
  return formatSize(bytes, useSIPrefix);
}
