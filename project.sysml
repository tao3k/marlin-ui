// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Project model entry following the deterministic-engineering architecture
// content contract. Load this file together with dev/sysml:
//   sysml -strict -validate -self-check -no-record-cache project.sysml dev/sysml
// Model map and requirement trace: dev/sysml/README.org
package MarlinUI {
    private import Views::*;

    part <'MUI-PROJECT'> project {
        doc /* Marlin UI product repository. Owns the native 120Hz Doom Emacs
               desktop workbench (internal/emacs/*), mobile companion, and Web
               WASM release targets. Consumes github.com/tao3k/marlin-mygo as a
               headless engine/protocol library and connects to marlin-agent-core
               over marlin.ui.v1 / marlin.app.v1 / marlin.gate.v1 gRPC. */
    }

    dependency productArchitecture from project to MarlinUIArchitecture::actual::releaseTargets;
    dependency hostTransportBoundary from project to MarlinUIArchitecture::actual::hostlinkTransport;
    dependency emacsWorkbenchDesign from project to MarlinEmacsWorkbench::actual::emacsFrame;
    dependency emacsTargetEvolution from project to MarlinEmacsWorkbench::target::emacsMultiWindow;
    dependency meowModalLifecycle from project to MarlinEmacsLifecycle::MeowModalLifecycle;
    dependency minibufferLifecycle from project to MarlinEmacsLifecycle::MinibufferLifecycle;

    view projectStructure {
        expose project;
        expose MarlinUIArchitecture::actual;
        expose MarlinUIArchitecture::target;
        expose MarlinUIArchitecture::requirements;
        render asTreeDiagram;
    }

    view hostTransportBoundaryView {
        expose MarlinUIArchitecture::actual::releaseTargets;
        expose MarlinUIArchitecture::actual::hostlinkTransport;
        expose MarlinUIArchitecture::actual::marlinMyGoEngine;
        expose MarlinUIArchitecture::actual::appConfigBoundary;
        expose MarlinUIArchitecture::actual::featureKitBoundary;
        expose MarlinUIArchitecture::actual::goBoundaryGuard;
        render asInterconnectionDiagram;
    }

    view emacsWorkbenchHierarchy {
        expose MarlinEmacsWorkbench::actual;
        expose MarlinEmacsWorkbench::target;
        expose MarlinEmacsWorkbench::requirements;
        render asTreeDiagram;
    }

    view emacsModalStateFlow {
        expose MarlinEmacsLifecycle::MeowModalLifecycle;
        render asTreeDiagram;
    }

    view emacsMinibufferStateFlow {
        expose MarlinEmacsLifecycle::MinibufferLifecycle;
        render asTreeDiagram;
    }
}
