import { gradeLabels, type FileSafety } from './safety';

export function SafetyDetail({ grade, reason }: FileSafety) {
  return (
    <div className="safety-detail">
      <strong className="safety-grade" data-grade={grade}>{grade} 级 · {gradeLabels[grade]}</strong>
      <span>{reason}</span>
    </div>
  );
}
