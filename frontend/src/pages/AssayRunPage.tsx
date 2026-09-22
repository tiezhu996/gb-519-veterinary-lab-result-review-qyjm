
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useAssayRunStore } from '../stores/assay-run';
export default function AssayRunPage() { return <EntityPage config={ENTITY_CONFIGS[2]} useStore={useAssayRunStore} showResultPanel />; }
