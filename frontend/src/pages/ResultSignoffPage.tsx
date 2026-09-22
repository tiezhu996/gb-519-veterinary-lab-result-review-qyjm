
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useResultSignoffStore } from '../stores/result-signoff';
export default function ResultSignoffPage() { return <EntityPage config={ENTITY_CONFIGS[3]} useStore={useResultSignoffStore} showResultPanel />; }
