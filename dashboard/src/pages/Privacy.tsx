import LegalDocument from '../components/LegalDocument';
import { LEGAL_VERSIONS, privacySections } from '../lib/legal';

export default function Privacy() { return <LegalDocument title="Privacy Notice" updated={LEGAL_VERSIONS.privacy} sections={privacySections} />; }
