export type OnboardingAnswers = {
  useCase: string;
  monthlyVisitors: string;
  website: string;
  botProblem: string;
  teamSize: string;
};

// stepComplete says whether a questionnaire step has everything it asks
// for. The steps are buttons, not form submits, so the browser's own
// `required` checks never run and would let a user skip every question.
export function stepComplete(step: number, a: OnboardingAnswers): boolean {
  switch (step) {
    case 1:
      return a.useCase !== '';
    case 2:
      return a.monthlyVisitors !== '' && a.teamSize !== '' && validWebsite(a.website);
    case 3:
      return a.botProblem !== '';
    default:
      return false;
  }
}

function validWebsite(value: string): boolean {
  try {
    const url = new URL(value.trim());
    return (url.protocol === 'https:' || url.protocol === 'http:') && url.hostname.includes('.');
  } catch {
    return false;
  }
}
