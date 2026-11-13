export const cleanupGrades = ['A', 'B', 'C', 'D'] as const;
export type CleanupGrade = typeof cleanupGrades[number];
export type CleanupGradeFilter = CleanupGrade | 'AB';

export interface FileSafety {
  grade: CleanupGrade;
  reason: string;
}

export const gradeLabels: Record<CleanupGrade, string> = {
  A: '低风险',
  B: '有条件清理',
  C: '逐项核对',
  D: '受保护',
};
