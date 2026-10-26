import { useEffect, useRef } from "react";
import { Compartment, EditorState } from "@codemirror/state";
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  drawSelection,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  defaultHighlightStyle,
  StreamLanguage,
  syntaxHighlighting,
} from "@codemirror/language";
import { python } from "@codemirror/lang-python";
import { go } from "@codemirror/legacy-modes/mode/go";
import type { Language } from "./api";

interface Props {
  code: string;
  language: Language;
  disabled: boolean;
  onChange: (code: string) => void;
}

export default function CodeEditor({
  code,
  language,
  disabled,
  onChange,
}: Props) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const languageConfig = useRef(new Compartment());
  const editableConfig = useRef(new Compartment());
  const initialCode = useRef(code);

  useEffect(() => {
    if (!host.current) return;
    const editor = new EditorView({
      parent: host.current,
      state: EditorState.create({
        doc: initialCode.current,
        extensions: [
          lineNumbers(),
          history(),
          drawSelection(),
          highlightActiveLine(),
          highlightActiveLineGutter(),
          keymap.of([...defaultKeymap, ...historyKeymap]),
          syntaxHighlighting(defaultHighlightStyle),
          languageConfig.current.of([]),
          editableConfig.current.of([]),
          EditorView.contentAttributes.of({
            "aria-label": "Code snippet",
            "aria-describedby": "input-hint",
            spellcheck: "false",
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) onChange(update.state.doc.toString());
          }),
        ],
      }),
    });
    view.current = editor;
    return () => {
      editor.destroy();
      view.current = null;
    };
  }, [onChange]);

  useEffect(() => {
    view.current?.dispatch({
      effects: languageConfig.current.reconfigure(
        language === "go" ? StreamLanguage.define(go) : python(),
      ),
    });
  }, [language]);

  useEffect(() => {
    view.current?.dispatch({
      effects: editableConfig.current.reconfigure([
        EditorState.readOnly.of(disabled),
        EditorView.editable.of(!disabled),
      ]),
    });
  }, [disabled]);

  useEffect(() => {
    const editor = view.current;
    if (editor && editor.state.doc.toString() !== code) {
      editor.dispatch({
        changes: { from: 0, to: editor.state.doc.length, insert: code },
      });
    }
  }, [code]);

  return (
    <div
      className={`code-editor${disabled ? " is-disabled" : ""}`}
      ref={host}
    />
  );
}
