// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Deterministic state machines for Meow modal editing and the Emacs Minibuffer.
package MarlinEmacsLifecycle {
    private import MarlinEmacsWorkbench::*;

    attribute def PressEscape;
    attribute def PressInsertKey;
    attribute def PressSpaceLeader;
    attribute def PressBufferPrefix;
    attribute def PressThemePrefix;
    attribute def ResolveKeypadAction;
    attribute def PressCancel;

    state def <'MUI-EMACS-L01'> MeowModalLifecycle {
        doc /* Meow modal editing state machine owned by internal/emacs/meow.
               C-g or ESC always returns deterministically to a safe resting
               state without trapping keyboard focus. */
        first start then insertMode;
        state insertMode;
        state normalMode;
        state keypadRoot;
        state keypadBuffer;
        state keypadTheme;

        transition enterNormal first insertMode accept PressEscape then normalMode;
        transition enterInsert first normalMode accept PressInsertKey then insertMode;
        transition openKeypad first normalMode accept PressSpaceLeader then keypadRoot;
        transition branchBuffer first keypadRoot accept PressBufferPrefix then keypadBuffer;
        transition branchTheme first keypadRoot accept PressThemePrefix then keypadTheme;
        transition completeFromRoot first keypadRoot accept ResolveKeypadAction then normalMode;
        transition completeFromBuffer first keypadBuffer accept ResolveKeypadAction then normalMode;
        transition completeFromTheme first keypadTheme accept ResolveKeypadAction then normalMode;
        transition cancelKeypadRoot first keypadRoot accept PressCancel then normalMode;
        transition cancelKeypadBuffer first keypadBuffer accept PressCancel then normalMode;
        transition cancelKeypadTheme first keypadTheme accept PressCancel then normalMode;
    }

    dependency meowLifecycleOwnedByMeow from MeowModalLifecycle to MarlinEmacsWorkbench::actual::emacsFrame::emacsMeow;

    attribute def InvokeMx;
    attribute def InvokeHelm;
    attribute def InvokeBufferSwitch;
    attribute def InvokeThemeSwitch;
    attribute def ConfirmCandidate;
    attribute def AbortMinibuffer;

    state def <'MUI-EMACS-L02'> MinibufferLifecycle {
        doc /* Interactive Emacs Minibuffer and completion lifecycle owned by
               internal/emacs/minibuffer. Only one completion mode is active at
               any time; confirming or cancelling always restores promptIdle. */
        first start then promptIdle;
        state promptIdle;
        state verticoCommand;
        state verticoBuffer;
        state verticoTheme;
        state helmSearch;

        transition openMx first promptIdle accept InvokeMx then verticoCommand;
        transition openHelm first promptIdle accept InvokeHelm then helmSearch;
        transition openBuffers first promptIdle accept InvokeBufferSwitch then verticoBuffer;
        transition openThemes first promptIdle accept InvokeThemeSwitch then verticoTheme;
        transition confirmMx first verticoCommand accept ConfirmCandidate then promptIdle;
        transition confirmBuffer first verticoBuffer accept ConfirmCandidate then promptIdle;
        transition confirmTheme first verticoTheme accept ConfirmCandidate then promptIdle;
        transition confirmHelm first helmSearch accept ConfirmCandidate then promptIdle;
        transition abortMx first verticoCommand accept AbortMinibuffer then promptIdle;
        transition abortBuffer first verticoBuffer accept AbortMinibuffer then promptIdle;
        transition abortTheme first verticoTheme accept AbortMinibuffer then promptIdle;
        transition abortHelm first helmSearch accept AbortMinibuffer then promptIdle;
    }

    dependency minibufferLifecycleOwnedByMinibuffer from MinibufferLifecycle to MarlinEmacsWorkbench::actual::emacsFrame::emacsMinibuffer;
}
