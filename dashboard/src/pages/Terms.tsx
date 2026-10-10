import LegalDocument from '../components/LegalDocument';
import { LEGAL_VERSIONS, termsSections } from '../lib/legal';

export default function Terms() { return <LegalDocument title="Terms of Service" updated={LEGAL_VERSIONS.terms} sections={termsSections} />; }
