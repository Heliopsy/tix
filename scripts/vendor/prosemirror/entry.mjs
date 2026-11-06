// SPDX-License-Identifier: AGPL-3.0-or-later

// What assets/richtext.js uses of ProseMirror and markdown-it, gathered on one
// global so the vendored file is a plain script, like every other asset.
export { Schema, Fragment, Slice } from "prosemirror-model";
export { EditorState, TextSelection, Plugin, PluginKey } from "prosemirror-state";
export { EditorView } from "prosemirror-view";
export { keymap } from "prosemirror-keymap";
export { history, undo, redo } from "prosemirror-history";
export { baseKeymap, toggleMark, setBlockType, wrapIn, lift, chainCommands, exitCode } from "prosemirror-commands";
export { wrapInList, splitListItem, liftListItem, sinkListItem } from "prosemirror-schema-list";
export { inputRules, InputRule, wrappingInputRule, textblockTypeInputRule } from "prosemirror-inputrules";
export { tableNodes, tableEditing, goToNextCell, addRowAfter, addColumnAfter, deleteRow, deleteColumn, deleteTable, isInTable } from "prosemirror-tables";
export { MarkdownParser, MarkdownSerializer, defaultMarkdownSerializer } from "prosemirror-markdown";
export { default as MarkdownIt } from "markdown-it";
