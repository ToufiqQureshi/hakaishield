import { validatePagesEnv } from './pages-env.mjs';

const errors = validatePagesEnv(process.env);
if (errors.length) {
  for (const error of errors) console.error(error);
  process.exitCode = 1;
}
