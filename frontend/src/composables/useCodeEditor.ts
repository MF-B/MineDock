import { ref, type Ref, onUnmounted } from "vue";
import { EditorView, basicSetup } from "codemirror";
import { EditorState } from "@codemirror/state";
import { oneDark } from "@codemirror/theme-one-dark";
import { json } from "@codemirror/lang-json";
import { yaml } from "@codemirror/lang-yaml";
import { xml } from "@codemirror/lang-xml";
import { javascript } from "@codemirror/lang-javascript";
import { markdown } from "@codemirror/lang-markdown";
import { java } from "@codemirror/lang-java";
import type { Extension } from "@codemirror/state";

function languageFromFilename(name: string): Extension | null {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  switch (ext) {
    case "json":
      return json();
    case "yml":
    case "yaml":
      return yaml();
    case "xml":
    case "html":
      return xml();
    case "js":
    case "ts":
    case "mjs":
    case "cjs":
      return javascript();
    case "md":
    case "markdown":
      return markdown();
    case "java":
    case "properties":
    case "cfg":
    case "conf":
    case "ini":
    case "toml":
      return java();
    default:
      return null;
  }
}

export function useCodeEditor() {
  const editorRef: Ref<HTMLElement | null> = ref(null);
  let view: EditorView | null = null;

  function create(content: string, filename: string, readOnly: boolean): void {
    destroy();
    if (!editorRef.value) return;

    const extensions: Extension[] = [basicSetup, oneDark];
    const lang = languageFromFilename(filename);
    if (lang) extensions.push(lang);
    if (readOnly) extensions.push(EditorState.readOnly.of(true));
    // Fill parent container height.
    extensions.push(
      EditorView.theme({
        "&": { height: "100%" },
        ".cm-scroller": { overflow: "auto" },
      }),
    );

    const state = EditorState.create({ doc: content, extensions });
    view = new EditorView({ state, parent: editorRef.value });
  }

  function getContent(): string {
    return view?.state.doc.toString() ?? "";
  }

  function destroy(): void {
    view?.destroy();
    view = null;
  }

  onUnmounted(destroy);

  return { editorRef, create, getContent, destroy };
}
