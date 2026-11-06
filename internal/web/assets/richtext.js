// SPDX-License-Identifier: AGPL-3.0-or-later

// The description editor's two modes: Markdown, the textarea as it always was,
// and Rich text, a ProseMirror view over the same text.
//
// The textarea stays the form field in both modes. Rich text writes the
// Markdown it serialises back into it on every change, so the server, the
// API and the history only ever see Markdown. A text the rich view would not
// give back as it found it is never opened there: check() parses it,
// serialises it and renders both through one markdown-it, and anything that
// renders differently keeps the Markdown mode with a notice saying why.
// Opening and closing the rich view writes nothing; only an edit does.
//
// The library (assets/prosemirror.min.js, built by scripts/vendor/prosemirror)
// is fetched the first time a page needs the rich view, so a page that never
// does pays nothing for it. Without a script, or without the library, the
// textarea is all there is, as before. The chosen mode is remembered in
// localStorage, per browser.
(function () {
  "use strict";

  var MODE_KEY = "tix.descriptionMode";
  var LIBRARY = "/assets/prosemirror.min.js";
  var SAFE_HREF = /^(https?:\/\/|mailto:)/i;

  function say(text) {
    return window.tix && window.tix.t ? window.tix.t(text) : text;
  }

  // taskLists is a markdown-it rule: a list item whose text starts [ ], [x]
  // or [X] and a space is a checklist item, as GitHub and the server read it.
  // The marker leaves the text and becomes the item's checked attribute.
  function taskLists(md) {
    md.core.ruler.after("inline", "tix_task_lists", function (state) {
      var tokens = state.tokens;
      for (var i = 2; i < tokens.length; i++) {
        var inline = tokens[i];
        if (inline.type !== "inline" || tokens[i - 1].type !== "paragraph_open" ||
          tokens[i - 2].type !== "list_item_open") {
          continue;
        }
        var m = /^\[([ xX])\](?:[ \t]+|$)/.exec(inline.content);
        var first = inline.children && inline.children[0];
        if (!m || !first || first.type !== "text" || first.content.indexOf(m[0]) !== 0) {
          continue;
        }
        first.content = first.content.slice(m[0].length);
        inline.content = inline.content.slice(m[0].length);
        var checked = m[1] !== " ";
        tokens[i - 2].meta = { checked: checked };
        tokens[i - 2].attrSet("data-task", checked ? "x" : " ");
      }
    });
  }

  function listIsTight(tokens, i) {
    while (++i < tokens.length) {
      if (tokens[i].type !== "list_item_open") return tokens[i].hidden;
    }
    return false;
  }

  function cellAlign(tok) {
    var m = /text-align:\s*(left|right|center)/.exec(tok.attrGet("style") || "");
    return { align: m ? m[1] : null };
  }

  function linkAttrs(tok) {
    var auto = tok.markup === "linkify" ? "bare" : tok.markup === "autolink" ? "angle" : null;
    return { href: tok.attrGet("href"), title: tok.attrGet("title") || null, auto: auto };
  }

  function cellSpec(tag, role) {
    return {
      content: "inline*",
      tableRole: role,
      isolating: true,
      attrs: { colspan: { default: 1 }, rowspan: { default: 1 }, colwidth: { default: null }, align: { default: null } },
      parseDOM: [{ tag: tag, getAttrs: function (dom) { return { align: dom.getAttribute("align") || null }; } }],
      toDOM: function (node) {
        return [tag, node.attrs.align ? { align: node.attrs.align } : {}, 0];
      }
    };
  }

  function schemaSpec() {
    return {
      nodes: {
        doc: { content: "block+" },
        paragraph: { content: "inline*", group: "block", parseDOM: [{ tag: "p" }], toDOM: function () { return ["p", 0]; } },
        blockquote: { content: "block+", group: "block", defining: true, parseDOM: [{ tag: "blockquote" }], toDOM: function () { return ["blockquote", 0]; } },
        horizontal_rule: { group: "block", parseDOM: [{ tag: "hr" }], toDOM: function () { return ["hr"]; } },
        heading: {
          attrs: { level: { default: 2 } },
          content: "inline*",
          group: "block",
          defining: true,
          parseDOM: [1, 2, 3, 4, 5, 6].map(function (n) { return { tag: "h" + n, attrs: { level: n } }; }),
          // Drawn two levels down, as the server draws a description, so
          // a heading looks here the way it will on the task screen.
          toDOM: function (node) { return ["h" + Math.min(node.attrs.level + 2, 6), 0]; }
        },
        code_block: {
          attrs: { params: { default: "" } },
          content: "text*",
          marks: "",
          group: "block",
          code: true,
          defining: true,
          parseDOM: [{ tag: "pre", preserveWhitespace: "full" }],
          toDOM: function () { return ["pre", ["code", 0]]; }
        },
        ordered_list: {
          attrs: { order: { default: 1 }, tight: { default: true } },
          content: "list_item+",
          group: "block",
          parseDOM: [{ tag: "ol", getAttrs: function (dom) { return { order: +(dom.getAttribute("start") || 1) }; } }],
          toDOM: function (node) { return ["ol", node.attrs.order === 1 ? {} : { start: node.attrs.order }, 0]; }
        },
        bullet_list: {
          attrs: { tight: { default: true }, bullet: { default: "-" } },
          content: "list_item+",
          group: "block",
          parseDOM: [{ tag: "ul" }],
          toDOM: function () { return ["ul", 0]; }
        },
        list_item: {
          attrs: { checked: { default: null } },
          content: "paragraph block*",
          defining: true,
          parseDOM: [{ tag: "li" }],
          toDOM: function (node) {
            return ["li", node.attrs.checked === null ? {} : { class: "task-item", "data-checked": String(node.attrs.checked) }, 0];
          }
        },
        table: {
          content: "table_row+",
          tableRole: "table",
          isolating: true,
          group: "block",
          parseDOM: [{ tag: "table" }],
          toDOM: function () { return ["table", ["tbody", 0]]; }
        },
        table_row: {
          content: "(table_cell | table_header)*",
          tableRole: "row",
          parseDOM: [{ tag: "tr" }],
          toDOM: function () { return ["tr", 0]; }
        },
        table_cell: cellSpec("td", "cell"),
        table_header: cellSpec("th", "header_cell"),
        text: { group: "inline" },
        image: {
          inline: true,
          attrs: { src: {}, alt: { default: null }, title: { default: null } },
          group: "inline",
          draggable: true,
          parseDOM: [{ tag: "img[src]", getAttrs: function (dom) {
            return { src: dom.getAttribute("src"), alt: dom.getAttribute("alt"), title: dom.getAttribute("title") };
          } }],
          // Never an <img>: the server links an image rather than loading
          // it, and so does the editor.
          toDOM: function (node) {
            return ["span", { class: "rt-image", title: node.attrs.src }, node.attrs.alt || node.attrs.src];
          }
        },
        hard_break: {
          inline: true,
          group: "inline",
          selectable: false,
          parseDOM: [{ tag: "br" }],
          toDOM: function () { return ["br"]; }
        }
      },
      marks: {
        link: {
          attrs: { href: {}, title: { default: null }, auto: { default: null } },
          inclusive: false,
          parseDOM: [{ tag: "a[href]", getAttrs: function (dom) {
            return { href: dom.getAttribute("href"), title: dom.getAttribute("title") };
          } }],
          toDOM: function (mark) {
            var attrs = { rel: "noopener noreferrer nofollow", title: mark.attrs.href };
            if (SAFE_HREF.test(mark.attrs.href)) attrs.href = mark.attrs.href;
            return ["a", attrs, 0];
          }
        },
        em: { parseDOM: [{ tag: "i" }, { tag: "em" }], toDOM: function () { return ["em", 0]; } },
        strong: { parseDOM: [{ tag: "strong" }, { tag: "b" }], toDOM: function () { return ["strong", 0]; } },
        strike: { parseDOM: [{ tag: "s" }, { tag: "del" }], toDOM: function () { return ["del", 0]; } },
        code: { parseDOM: [{ tag: "code" }], toDOM: function () { return ["code", 0]; } }
      }
    };
  }

  function tokenSpec() {
    return {
      blockquote: { block: "blockquote" },
      paragraph: { block: "paragraph" },
      list_item: { block: "list_item", getAttrs: function (tok) {
        return { checked: tok.meta && typeof tok.meta.checked === "boolean" ? tok.meta.checked : null };
      } },
      bullet_list: { block: "bullet_list", getAttrs: function (tok, tokens, i) {
        return { tight: listIsTight(tokens, i), bullet: tok.markup || "-" };
      } },
      ordered_list: { block: "ordered_list", getAttrs: function (tok, tokens, i) {
        return { order: +tok.attrGet("start") || 1, tight: listIsTight(tokens, i) };
      } },
      heading: { block: "heading", getAttrs: function (tok) { return { level: +tok.tag.slice(1) }; } },
      code_block: { block: "code_block", noCloseToken: true },
      fence: { block: "code_block", getAttrs: function (tok) { return { params: tok.info || "" }; }, noCloseToken: true },
      hr: { node: "horizontal_rule" },
      image: { node: "image", getAttrs: function (tok) {
        return { src: tok.attrGet("src"), title: tok.attrGet("title") || null, alt: (tok.children[0] && tok.children[0].content) || null };
      } },
      hardbreak: { node: "hard_break" },
      // The server draws every line break as one (hard wraps), so a soft
      // break is a hard one here too, and is written back as a newline.
      softbreak: { node: "hard_break" },
      em: { mark: "em" },
      strong: { mark: "strong" },
      s: { mark: "strike" },
      link: { mark: "link", getAttrs: linkAttrs },
      code_inline: { mark: "code", noCloseToken: true },
      table: { block: "table" },
      thead: { ignore: true },
      tbody: { ignore: true },
      tr: { block: "table_row" },
      th: { block: "table_header", getAttrs: cellAlign },
      td: { block: "table_cell", getAttrs: cellAlign }
    };
  }

  function box(item) {
    return item.attrs.checked === null ? "" : item.attrs.checked ? "[x] " : "[ ] ";
  }

  function isPlainURL(link, parent, index) {
    if (link.attrs.title || !/^\w+:/.test(link.attrs.href)) return false;
    var content = parent.child(index);
    if (!content.isText || content.text !== link.attrs.href || content.marks[content.marks.length - 1] !== link) return false;
    return index === parent.childCount - 1 || !link.isInSet(parent.child(index + 1).marks);
  }

  // isBareLink is a link markdown-it found in plain text: written back as the
  // text it was, so the address is never escaped or bracketed.
  function isBareLink(link, parent, index) {
    if (link.attrs.auto !== "bare") return false;
    var content = parent.child(index);
    if (!content.isText || content.marks[content.marks.length - 1] !== link) return false;
    return index === parent.childCount - 1 || !link.isInSet(parent.child(index + 1).marks);
  }

  function inlineBreaks(parent) {
    return parent.type.name === "heading" || parent.type.spec.tableRole === "cell" ||
      parent.type.spec.tableRole === "header_cell";
  }

  function serializerSpec(PM, cellText) {
    var base = PM.defaultMarkdownSerializer;
    var nodes = {
      blockquote: base.nodes.blockquote,
      heading: base.nodes.heading,
      horizontal_rule: base.nodes.horizontal_rule,
      paragraph: base.nodes.paragraph,
      list_item: base.nodes.list_item,
      image: base.nodes.image,
      code_block: function (state, node) {
        var ticks = node.textContent.match(/`{3,}/gm);
        var fence = ticks ? ticks.sort().slice(-1)[0] + "`" : "```";
        state.write(fence + (node.attrs.params || "") + "\n");
        if (node.textContent) {
          state.text(node.textContent, false);
          state.write("\n");
        }
        state.write(fence);
        state.closeBlock(node);
      },
      bullet_list: function (state, node) {
        state.renderList(node, "  ", function (i) { return (node.attrs.bullet || "-") + " " + box(node.child(i)); });
      },
      ordered_list: function (state, node) {
        var start = node.attrs.order || 1;
        var width = String(start + node.childCount - 1).length;
        state.renderList(node, state.repeat(" ", width + 2), function (i) {
          var n = String(start + i);
          return state.repeat(" ", width - n.length) + n + ". " + box(node.child(i));
        });
      },
      hard_break: function (state, node, parent, index) {
        for (var i = index + 1; i < parent.childCount; i++) {
          if (parent.child(i).type !== node.type) {
            if (inlineBreaks(parent)) {
              state.write(" ");
            } else {
              state.write("\n");
              state.afterBreak = true;
            }
            return;
          }
        }
      },
      text: function (state, node) {
        if (state.afterBreak) {
          state.atBlockStart = true;
          state.afterBreak = false;
        }
        state.text(node.text, !state.inAutolink);
      },
      table: function (state, node) {
        var rows = [];
        node.forEach(function (row) {
          var cells = [];
          row.forEach(function (cell) { cells.push(cellText(cell)); });
          rows.push(cells);
        });
        var width = 0;
        rows.forEach(function (r) { width = Math.max(width, r.length); });
        var header = node.firstChild;
        var rule = [];
        for (var c = 0; c < width; c++) {
          var align = c < header.childCount ? header.child(c).attrs.align : null;
          rule.push(align === "center" ? ":---:" : align === "right" ? "---:" : align === "left" ? ":---" : "---");
        }
        function line(cells) {
          var out = [];
          for (var k = 0; k < width; k++) out.push(cells[k] || "");
          state.write("| " + out.join(" | ") + " |");
          state.ensureNewLine();
        }
        line(rows[0]);
        line(rule);
        for (var r = 1; r < rows.length; r++) line(rows[r]);
        state.closeBlock(node);
      }
    };
    var marks = {
      em: base.marks.em,
      strong: base.marks.strong,
      code: base.marks.code,
      strike: { open: "~~", close: "~~", mixable: true, expelEnclosingWhitespace: true },
      link: {
        open: function (state, mark, parent, index) {
          state.linkKind = isBareLink(mark, parent, index) ? "bare" : isPlainURL(mark, parent, index) ? "angle" : null;
          state.inAutolink = state.linkKind !== null;
          return state.linkKind === "bare" ? "" : state.linkKind === "angle" ? "<" : "[";
        },
        close: function (state, mark) {
          var kind = state.linkKind;
          state.linkKind = null;
          state.inAutolink = undefined;
          if (kind === "bare") return "";
          if (kind === "angle") return ">";
          return "](" + mark.attrs.href.replace(/[()"\s]/g, function (c) { return c === " " ? "%20" : "\\" + c; }) +
            (mark.attrs.title ? ' "' + mark.attrs.title.replace(/"/g, '\\"') + '"' : "") + ")";
        },
        mixable: true
      }
    };
    return { nodes: nodes, marks: marks };
  }

  // codec builds the parser, serialiser and fidelity check over the
  // ProseMirror and markdown-it the vendored file defines.
  function codec(PM) {
    var schema = new PM.Schema(schemaSpec());
    var md = new PM.MarkdownIt("default", { html: false, linkify: true, breaks: true });
    md.use(taskLists);
    var parser = new PM.MarkdownParser(schema, md, tokenSpec());
    var serializer;
    function cellText(cell) {
      var text = serializer.serialize(schema.node("doc", null, [schema.node("paragraph", null, cell.content)]));
      return text.replace(/\n/g, " ").replace(/\|/g, "\\|");
    }
    var spec = serializerSpec(PM, cellText);
    serializer = new PM.MarkdownSerializer(spec.nodes, spec.marks, { tightLists: true });

    function parse(text) {
      return parser.parse(String(text || ""));
    }

    function serialize(doc) {
      return serializer.serialize(doc).replace(/\n+$/, "");
    }

    // check says whether text survives the rich view: parsed and written
    // back, it must render exactly as it did. reason names what failed.
    function check(text) {
      var doc;
      try {
        doc = parse(text);
      } catch (e) {
        return { ok: false, reason: "parse" };
      }
      var back;
      try {
        back = serialize(doc);
      } catch (e) {
        return { ok: false, reason: "serialize" };
      }
      if (md.render(String(text || "")) !== md.render(back)) {
        return { ok: false, reason: "render", doc: doc, markdown: back };
      }
      if (looseTilde(doc)) {
        return { ok: false, reason: "tilde", doc: doc, markdown: back };
      }
      return { ok: true, doc: doc, markdown: back };
    }

    // looseTilde finds a ~ left in plain text. The server strikes through
    // ~one tilde~ as well as ~~two~~, and markdown-it only the second, so the
    // check above cannot see that a single-tilde pair would be lost.
    function looseTilde(doc) {
      var found = false;
      doc.descendants(function (node) {
        if (found) return false;
        if (node.isText && node.text.indexOf("~") !== -1 && !schema.marks.code.isInSet(node.marks)) found = true;
        return !node.type.spec.code;
      });
      return found;
    }

    return { schema: schema, markdownIt: md, parse: parse, serialize: serialize, check: check };
  }

  // storedMode is the mode this browser last chose, "source" when it never
  // chose or storage is refused.
  function storedMode(storage) {
    try {
      return storage && storage.getItem(MODE_KEY) === "rich" ? "rich" : "source";
    } catch (e) {
      return "source";
    }
  }

  function storeMode(storage, mode) {
    try {
      if (storage) storage.setItem(MODE_KEY, mode);
    } catch (e) {
      return;
    }
  }

  window.tixRich = { codec: codec, storedMode: storedMode, storeMode: storeMode, MODE_KEY: MODE_KEY };

  if (typeof document === "undefined" || !document.addEventListener) return;

  var library = null;
  var shared = null;

  function storage() {
    try {
      return window.localStorage;
    } catch (e) {
      return null;
    }
  }

  // load fetches the vendored library once and builds the codec over it.
  function load() {
    if (!library) {
      library = new Promise(function (resolve, reject) {
        if (window.tixPM) {
          resolve(window.tixPM);
          return;
        }
        var script = document.createElement("script");
        script.src = LIBRARY;
        script.onload = function () {
          if (window.tixPM) resolve(window.tixPM);
          else reject(new Error("no library"));
        };
        script.onerror = function () {
          library = null;
          reject(new Error("no library"));
        };
        document.head.appendChild(script);
      }).then(function (PM) {
        shared = { PM: PM, codec: codec(PM) };
        return shared;
      });
    }
    return library;
  }

  function icon(name) {
    var ns = "http://www.w3.org/2000/svg";
    var svg = document.createElementNS(ns, "svg");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("width", "16");
    svg.setAttribute("height", "16");
    var use = document.createElementNS(ns, "use");
    use.setAttribute("href", "/assets/icons.svg#" + name);
    svg.appendChild(use);
    return svg;
  }

  function button(cls, label, iconName) {
    var b = document.createElement("button");
    b.type = "button";
    b.className = cls;
    if (iconName) {
      b.appendChild(icon(iconName));
      b.setAttribute("aria-label", label);
      b.title = label;
    } else {
      b.textContent = label;
    }
    // Pressing a tool must not take the caret out of the text.
    b.addEventListener("mousedown", function (event) { event.preventDefault(); });
    return b;
  }

  function ancestor($pos, type) {
    for (var d = $pos.depth; d > 0; d--) {
      if ($pos.node(d).type === type) return { node: $pos.node(d), pos: $pos.before(d), depth: d };
    }
    return null;
  }

  function markActive(state, type) {
    var sel = state.selection;
    if (sel.empty) return !!type.isInSet(state.storedMarks || sel.$from.marks());
    return state.doc.rangeHasMark(sel.from, sel.to, type);
  }

  // commands are the toolbar's and keyboard's edits over one schema.
  function commands(PM, schema) {
    var n = schema.nodes;
    var m = schema.marks;

    function toggleList(type) {
      return function (state, dispatch) {
        var list = ancestor(state.selection.$from, type);
        if (list) return PM.liftListItem(n.list_item)(state, dispatch);
        return PM.wrapInList(type)(state, dispatch);
      };
    }

    function checklist(state, dispatch) {
      var item = ancestor(state.selection.$from, n.list_item);
      if (item) {
        if (dispatch) {
          dispatch(state.tr.setNodeMarkup(item.pos, null, { checked: item.node.attrs.checked === null ? false : null }));
        }
        return true;
      }
      return PM.wrapInList(n.bullet_list)(state, dispatch && function (tr) {
        var list = ancestor(tr.selection.$from, n.bullet_list);
        if (list) {
          list.node.forEach(function (child, offset) {
            tr.setNodeMarkup(list.pos + 1 + offset, null, { checked: false });
          });
        }
        dispatch(tr);
      });
    }

    function heading(state, dispatch) {
      if (state.selection.$from.parent.type === n.heading) return PM.setBlockType(n.paragraph)(state, dispatch);
      return PM.setBlockType(n.heading, { level: 2 })(state, dispatch);
    }

    function quote(state, dispatch) {
      if (ancestor(state.selection.$from, n.blockquote)) return PM.lift(state, dispatch);
      return PM.wrapIn(n.blockquote)(state, dispatch);
    }

    function code(state, dispatch) {
      var sel = state.selection;
      if (sel.$from.parent.type === n.code_block) return PM.setBlockType(n.paragraph)(state, dispatch);
      if (sel.empty || !sel.$from.sameParent(sel.$to)) return PM.setBlockType(n.code_block)(state, dispatch);
      return PM.toggleMark(m.code)(state, dispatch);
    }

    function table(state, dispatch) {
      if (PM.isInTable(state)) return false;
      if (dispatch) {
        var row = function (cell) { return n.table_row.create(null, [cell.create(), cell.create()]); };
        var node = n.table.create(null, [row(n.table_header), row(n.table_cell)]);
        dispatch(state.tr.replaceSelectionWith(node).scrollIntoView());
      }
      return true;
    }

    function hardBreak(state, dispatch) {
      if (dispatch) dispatch(state.tr.replaceSelectionWith(n.hard_break.create()).scrollIntoView());
      return true;
    }

    function splitItem(state, dispatch) {
      var item = ancestor(state.selection.$from, n.list_item);
      var attrs = item && item.node.attrs.checked !== null ? { checked: false } : { checked: null };
      return PM.splitListItem(n.list_item, attrs)(state, dispatch);
    }

    return {
      bold: PM.toggleMark(m.strong),
      italic: PM.toggleMark(m.em),
      strike: PM.toggleMark(m.strike),
      heading: heading,
      bullets: toggleList(n.bullet_list),
      numbers: toggleList(n.ordered_list),
      checklist: checklist,
      quote: quote,
      code: code,
      table: table,
      hardBreak: PM.chainCommands(PM.exitCode, hardBreak),
      splitItem: splitItem,
      sink: PM.sinkListItem(n.list_item),
      lift: PM.liftListItem(n.list_item)
    };
  }

  function inputRules(PM, schema) {
    var n = schema.nodes;
    var box = new PM.InputRule(/^\[([ xX])\]\s$/, function (state, match, start, end) {
      var $start = state.doc.resolve(start);
      var item = ancestor($start, n.list_item);
      if (!item || $start.index(item.depth) !== 0 || $start.parentOffset !== 0) return null;
      return state.tr.delete(start, end).setNodeMarkup(item.pos, null, { checked: match[1] !== " " });
    });
    return PM.inputRules({
      rules: [
        PM.textblockTypeInputRule(/^(#{1,6})\s$/, n.heading, function (match) { return { level: match[1].length }; }),
        PM.wrappingInputRule(/^\s*>\s$/, n.blockquote),
        PM.wrappingInputRule(/^\s*([-+*])\s$/, n.bullet_list, function (match) { return { bullet: match[1] }; }),
        PM.wrappingInputRule(/^(\d+)\.\s$/, n.ordered_list, function (match) { return { order: +match[1] }; },
          function (match, node) { return node.childCount + node.attrs.order === +match[1]; }),
        PM.textblockTypeInputRule(/^```([\w-]*)\s$/, n.code_block, function (match) { return { params: match[1] }; }),
        box
      ]
    });
  }

  // checkItem draws a checklist item with a real checkbox in front of its
  // text; ticking it is an edit like any other.
  function checkItem(node, view, getPos) {
    var li = document.createElement("li");
    var content = document.createElement("div");
    content.className = "rt-taskbody";
    var input = null;
    function draw() {
      if (node.attrs.checked === null) {
        li.className = "";
        if (input) {
          li.removeChild(input);
          input = null;
        }
        return;
      }
      li.className = "task-item rt-task";
      if (!input) {
        input = document.createElement("input");
        input.type = "checkbox";
        input.contentEditable = "false";
        input.setAttribute("aria-label", say("Checklist item"));
        input.addEventListener("mousedown", function (event) { event.preventDefault(); });
        input.addEventListener("click", function (event) {
          event.preventDefault();
          var pos = getPos();
          if (typeof pos !== "number") return;
          var current = view.state.doc.nodeAt(pos);
          view.dispatch(view.state.tr.setNodeMarkup(pos, null, { checked: !current.attrs.checked }));
        });
        li.insertBefore(input, content);
      }
      input.checked = !!node.attrs.checked;
    }
    li.appendChild(content);
    draw();
    return {
      dom: li,
      contentDOM: content,
      update: function (next) {
        if (next.type !== node.type) return false;
        node = next;
        draw();
        return true;
      },
      stopEvent: function (event) { return !!input && event.target === input; },
      ignoreMutation: function (mutation) { return !!input && (mutation.target === input || mutation.target === li); }
    };
  }

  // mentions is @handle completion inside the rich view: the same matching
  // decide.js gives the textarea (assets/mention.js), with a listbox drawn
  // under the editor.
  function mentions(PM, ed) {
    var tix = window.tix;
    var open = null;

    function handles() {
      var list = document.getElementById(ed.area.getAttribute("data-mention") || "");
      var out = [];
      if (!list) return out;
      for (var i = 0; i < list.options.length; i++) {
        if (list.options[i].value) out.push(list.options[i].value);
      }
      return out;
    }

    function close() {
      if (!open) return;
      if (open.menu.parentNode) open.menu.parentNode.removeChild(open.menu);
      if (ed.view) {
        ed.view.dom.setAttribute("aria-expanded", "false");
        ed.view.dom.removeAttribute("aria-activedescendant");
      }
      open = null;
    }

    function highlight(index) {
      var items = open.menu.querySelectorAll("[role=option]");
      open.index = (index + items.length) % items.length;
      for (var i = 0; i < items.length; i++) {
        items[i].setAttribute("aria-selected", i === open.index ? "true" : "false");
      }
      ed.view.dom.setAttribute("aria-activedescendant", items[open.index].id);
    }

    function choose(handle) {
      var view = ed.view;
      var from = open.from;
      var to = open.to;
      close();
      view.dispatch(view.state.tr.insertText("@" + handle + " ", from, to));
      view.focus();
    }

    function update(view) {
      var sel = view.state.selection;
      var $from = sel.$from;
      if (!tix || !tix.mentionAt || !sel.empty || !$from.parent.isTextblock || $from.parent.type.spec.code ||
        !view.hasFocus()) {
        close();
        return;
      }
      var before = $from.parent.textBetween(0, $from.parentOffset, "\n", "￼");
      var at = tix.mentionAt(before, before.length);
      var matches = at ? tix.mentionMatches(handles(), at.query, 8) : [];
      if (!matches.length) {
        close();
        return;
      }
      if (!open) {
        var menu = document.createElement("ul");
        menu.className = "mention-menu";
        menu.id = ed.area.id + "-rich-mentions";
        menu.setAttribute("role", "listbox");
        menu.setAttribute("aria-label", say("People to mention"));
        menu.addEventListener("mousedown", function (event) {
          var item = event.target.closest && event.target.closest("[role=option]");
          event.preventDefault();
          if (item) choose(item.getAttribute("data-handle"));
        });
        ed.mount.insertAdjacentElement("afterend", menu);
        view.dom.setAttribute("aria-controls", menu.id);
        view.dom.setAttribute("aria-autocomplete", "list");
        open = { menu: menu, index: 0 };
      }
      open.from = $from.start() + at.start;
      open.to = $from.pos;
      open.menu.innerHTML = "";
      for (var i = 0; i < matches.length; i++) {
        var li = document.createElement("li");
        li.id = open.menu.id + "-" + i;
        li.setAttribute("role", "option");
        li.setAttribute("data-handle", matches[i]);
        li.textContent = "@" + matches[i];
        open.menu.appendChild(li);
      }
      view.dom.setAttribute("aria-expanded", "true");
      highlight(0);
    }

    function key(view, event) {
      if (!open) return false;
      var k = event.key;
      if (k === "ArrowDown" || k === "ArrowUp") {
        highlight(open.index + (k === "ArrowDown" ? 1 : -1));
      } else if ((k === "Enter" || k === "Tab") && !event.shiftKey && !event.ctrlKey && !event.metaKey) {
        var item = open.menu.querySelectorAll("[role=option]")[open.index];
        choose(item.getAttribute("data-handle"));
      } else if (k === "Escape") {
        close();
      } else {
        return false;
      }
      event.preventDefault();
      event.stopPropagation();
      return true;
    }

    return new PM.Plugin({
      view: function () { return { update: update, destroy: close }; },
      props: {
        handleKeyDown: key,
        handleDOMEvents: { blur: function () { close(); return false; } }
      }
    });
  }

  // tools are the toolbar's buttons: what each is called, its icon, the
  // command it runs and, for a mark or block, how to tell it is on.
  function tools(schema, cmd) {
    var n = schema.nodes;
    var m = schema.marks;
    function inBlock(type) {
      return function (state) { return !!ancestor(state.selection.$from, type) || state.selection.$from.parent.type === type; };
    }
    return [
      { name: "bold", label: say("Bold"), icon: "fmt-bold", run: cmd.bold, on: function (s) { return markActive(s, m.strong); } },
      { name: "italic", label: say("Italic"), icon: "fmt-italic", run: cmd.italic, on: function (s) { return markActive(s, m.em); } },
      { name: "strike", label: say("Strikethrough"), icon: "fmt-strike", run: cmd.strike, on: function (s) { return markActive(s, m.strike); } },
      { name: "heading", label: say("Heading"), icon: "fmt-heading", run: cmd.heading, on: inBlock(n.heading) },
      { name: "bullets", label: say("Bulleted list"), icon: "fmt-list", run: cmd.bullets, on: inBlock(n.bullet_list) },
      { name: "numbers", label: say("Numbered list"), icon: "fmt-numbers", run: cmd.numbers, on: inBlock(n.ordered_list) },
      { name: "checklist", label: say("Checklist"), icon: "checklist", run: cmd.checklist, on: function (s) {
        var item = ancestor(s.selection.$from, n.list_item);
        return !!item && item.node.attrs.checked !== null;
      } },
      { name: "quote", label: say("Quote"), icon: "fmt-quote", run: cmd.quote, on: inBlock(n.blockquote) },
      { name: "code", label: say("Code"), icon: "pi-code", run: cmd.code, on: function (s) {
        return s.selection.$from.parent.type === n.code_block || markActive(s, m.code);
      } },
      { name: "link", label: say("Link"), icon: "fmt-link", on: function (s) { return markActive(s, m.link); } },
      { name: "table", label: say("Table"), icon: "fmt-table", run: cmd.table, on: function () { return false; } }
    ];
  }

  function linkRange(state, type) {
    var sel = state.selection;
    if (!sel.empty) return { from: sel.from, to: sel.to };
    var $pos = sel.$from;
    var parent = $pos.parent;
    var start = $pos.index();
    var end = start;
    var at = $pos.start();
    if (start >= parent.childCount || !type.isInSet(parent.child(start).marks)) {
      if (start === 0 || !type.isInSet(parent.child(start - 1).marks)) return null;
      start--;
      end--;
    }
    while (start > 0 && type.isInSet(parent.child(start - 1).marks)) start--;
    while (end + 1 < parent.childCount && type.isInSet(parent.child(end + 1).marks)) end++;
    var from = at;
    for (var i = 0; i < start; i++) from += parent.child(i).nodeSize;
    var to = from;
    for (var j = start; j <= end; j++) to += parent.child(j).nodeSize;
    return { from: from, to: to };
  }

  // Editor is one description field with its mode switch.
  function Editor(area) {
    this.area = area;
    this.view = null;
    this.busy = false;
    this.build();
  }

  Editor.prototype.build = function () {
    var ed = this;
    var area = this.area;
    var modes = document.createElement("div");
    modes.className = "segmented is-small rt-modes";
    modes.setAttribute("role", "group");
    modes.setAttribute("aria-label", say("Editor"));
    this.richButton = button("segment", say("Rich text"));
    this.sourceButton = button("segment", say("Markdown"));
    this.richButton.addEventListener("click", function () { ed.setMode("rich", true); });
    this.sourceButton.addEventListener("click", function () { ed.setMode("source", true); });
    modes.appendChild(this.richButton);
    modes.appendChild(this.sourceButton);
    var label = area.id ? document.querySelector('label[for="' + area.id + '"]') : null;
    var row = label && label.parentNode;
    if (row && row.classList && (row.classList.contains("field-label") || row.classList.contains("cfield-label"))) {
      row.classList.add("has-modes");
      row.appendChild(modes);
    } else {
      area.insertAdjacentElement("beforebegin", modes);
    }

    this.wrap = document.createElement("div");
    this.wrap.className = "rt";
    this.wrap.hidden = true;
    this.toolbar = document.createElement("div");
    this.toolbar.className = "rt-toolbar";
    this.toolbar.setAttribute("role", "toolbar");
    this.toolbar.setAttribute("aria-label", say("Formatting"));
    this.tablebar = document.createElement("div");
    this.tablebar.className = "rt-subbar";
    this.tablebar.hidden = true;
    this.linkbar = document.createElement("div");
    this.linkbar.className = "rt-subbar rt-linkbar";
    this.linkbar.hidden = true;
    this.mount = document.createElement("div");
    this.mount.className = "rt-mount";
    this.wrap.appendChild(this.toolbar);
    this.wrap.appendChild(this.tablebar);
    this.wrap.appendChild(this.linkbar);
    this.wrap.appendChild(this.mount);
    area.insertAdjacentElement("afterend", this.wrap);

    this.note = document.createElement("p");
    this.note.className = "rt-note";
    this.note.setAttribute("role", "status");
    this.note.hidden = true;
    this.wrap.insertAdjacentElement("afterend", this.note);
    this.paint("source");
  };

  Editor.prototype.paint = function (mode) {
    this.mode = mode;
    this.richButton.setAttribute("aria-pressed", mode === "rich" ? "true" : "false");
    this.sourceButton.setAttribute("aria-pressed", mode === "source" ? "true" : "false");
    this.richButton.classList.toggle("is-on", mode === "rich");
    this.sourceButton.classList.toggle("is-on", mode === "source");
    var preview = this.area.id ? document.querySelector('[data-md-preview="' + this.area.id + '"]') : null;
    if (preview) {
      if (mode === "rich" && preview.getAttribute("aria-pressed") === "true") preview.click();
      preview.hidden = mode === "rich";
    }
  };

  Editor.prototype.say = function (text) {
    this.note.textContent = text ? say(text) : "";
    this.note.hidden = !text;
  };

  Editor.prototype.setMode = function (mode, chosen) {
    var ed = this;
    if (chosen) storeMode(storage(), mode);
    if (mode === "source") {
      this.toSource(chosen);
      return;
    }
    if (this.view || this.busy) return;
    this.busy = true;
    load().then(function (lib) {
      ed.busy = false;
      if (!ed.area.isConnected) return;
      ed.toRich(lib, chosen);
    }, function () {
      ed.busy = false;
      ed.say("The rich text editor could not be loaded. Your text is unchanged.");
    });
  };

  Editor.prototype.toSource = function (focus) {
    if (this.view) {
      this.view.destroy();
      this.view = null;
      this.mount.innerHTML = "";
    }
    this.wrap.hidden = true;
    this.area.hidden = false;
    this.say("");
    this.paint("source");
    if (focus) this.area.focus();
  };

  Editor.prototype.toRich = function (lib, focus) {
    var ed = this;
    var PM = lib.PM;
    var c = lib.codec;
    var checked = c.check(this.area.value);
    if (!checked.ok) {
      this.say("This text uses Markdown the rich text editor cannot keep exactly, so it stays in Markdown.");
      this.paint("source");
      return;
    }
    var cmd = commands(PM, c.schema);
    this.cmd = cmd;
    this.tools = tools(c.schema, cmd);
    this.drawToolbar(PM, c);
    var state = PM.EditorState.create({
      doc: checked.doc,
      plugins: [
        mentions(PM, this),
        inputRules(PM, c.schema),
        PM.keymap({
          "Mod-z": PM.undo,
          "Shift-Mod-z": PM.redo,
          "Mod-y": PM.redo,
          "Mod-b": cmd.bold,
          "Mod-i": cmd.italic,
          "Mod-`": cmd.code,
          "Mod-Enter": function () {
            if (ed.area.form && ed.area.form.requestSubmit) ed.area.form.requestSubmit();
            return true;
          },
          "Shift-Enter": cmd.hardBreak,
          "Enter": cmd.splitItem,
          "Tab": PM.chainCommands(PM.goToNextCell(1), cmd.sink),
          "Shift-Tab": PM.chainCommands(PM.goToNextCell(-1), cmd.lift)
        }),
        PM.keymap(PM.baseKeymap),
        PM.history(),
        PM.tableEditing()
      ]
    });
    var label = this.area.id ? document.querySelector('label[for="' + this.area.id + '"]') : null;
    this.view = new PM.EditorView(this.mount, {
      state: state,
      nodeViews: { list_item: checkItem },
      attributes: function (current) {
        var doc = current.doc;
        var empty = doc.childCount === 1 && doc.firstChild.type === c.schema.nodes.paragraph && doc.firstChild.content.size === 0;
        return {
          class: "md rt-editor" + (empty ? " is-empty" : ""),
          role: "textbox",
          "aria-multiline": "true",
          "aria-label": label ? label.textContent.trim() : say("Description"),
          "data-placeholder": ed.area.getAttribute("placeholder") || ""
        };
      },
      clipboardTextParser: function (text, $context, plain) {
        if (plain || $context.parent.type.spec.code) return null;
        var parsed = c.check(text);
        return parsed.ok ? PM.Slice.maxOpen(parsed.doc.content) : null;
      },
      dispatchTransaction: function (tr) {
        var view = ed.view;
        view.updateState(view.state.apply(tr));
        if (tr.docChanged) {
          ed.area.value = c.serialize(view.state.doc);
          ed.area.dispatchEvent(new Event("input", { bubbles: true }));
        }
        ed.refresh();
      }
    });
    this.area.hidden = true;
    this.wrap.hidden = false;
    this.say("");
    this.paint("rich");
    this.refresh();
    if (focus) this.view.focus();
  };

  Editor.prototype.drawToolbar = function (PM, c) {
    var ed = this;
    this.toolbar.innerHTML = "";
    this.buttons = {};
    this.tools.forEach(function (tool) {
      var b = button("rt-btn", tool.label, tool.icon);
      b.setAttribute("aria-pressed", "false");
      b.addEventListener("click", function () {
        if (!ed.view) return;
        if (tool.name === "link") {
          ed.toggleLink(PM, c);
          return;
        }
        tool.run(ed.view.state, ed.view.dispatch);
        ed.view.focus();
      });
      ed.buttons[tool.name] = b;
      ed.toolbar.appendChild(b);
    });

    this.tablebar.innerHTML = "";
    [[say("Add row"), PM.addRowAfter], [say("Add column"), PM.addColumnAfter], [say("Delete row"), PM.deleteRow],
      [say("Delete column"), PM.deleteColumn], [say("Delete table"), PM.deleteTable]].forEach(function (pair) {
      var b = button("link rt-textbtn", pair[0]);
      b.addEventListener("click", function () {
        if (!ed.view) return;
        pair[1](ed.view.state, ed.view.dispatch);
        ed.view.focus();
      });
      ed.tablebar.appendChild(b);
    });

    this.linkbar.innerHTML = "";
    var input = document.createElement("input");
    input.type = "url";
    input.className = "rt-linkinput";
    input.placeholder = "https://";
    input.setAttribute("aria-label", say("Link address"));
    var apply = button("rt-textbtn", say("Add link"));
    var cancel = button("link rt-textbtn", say("Cancel"));
    function done() {
      ed.linkbar.hidden = true;
      ed.view.focus();
    }
    function commit() {
      var href = input.value.trim();
      if (!href || !ed.view) {
        done();
        return;
      }
      var state = ed.view.state;
      var type = c.schema.marks.link;
      var sel = state.selection;
      var tr = state.tr;
      if (sel.empty) {
        tr.insertText(href, sel.from).addMark(sel.from, sel.from + href.length, type.create({ href: href }));
      } else {
        tr.addMark(sel.from, sel.to, type.create({ href: href }));
      }
      ed.view.dispatch(tr);
      input.value = "";
      done();
    }
    input.addEventListener("keydown", function (event) {
      if (event.key === "Enter") {
        event.preventDefault();
        commit();
      } else if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        done();
      }
    });
    apply.addEventListener("click", commit);
    cancel.addEventListener("click", done);
    this.linkbar.appendChild(input);
    this.linkbar.appendChild(apply);
    this.linkbar.appendChild(cancel);
    this.linkInput = input;
  };

  Editor.prototype.toggleLink = function (PM, c) {
    var state = this.view.state;
    var type = c.schema.marks.link;
    if (markActive(state, type)) {
      var range = linkRange(state, type);
      if (range) this.view.dispatch(state.tr.removeMark(range.from, range.to, type));
      this.view.focus();
      return;
    }
    this.linkbar.hidden = false;
    this.linkInput.focus();
  };

  Editor.prototype.refresh = function () {
    if (!this.view) return;
    var state = this.view.state;
    var ed = this;
    this.tools.forEach(function (tool) {
      var b = ed.buttons[tool.name];
      b.setAttribute("aria-pressed", tool.on(state) ? "true" : "false");
      b.disabled = tool.run ? !tool.run(state) : false;
    });
    var inTable = shared.PM.isInTable(state);
    this.tablebar.hidden = !inTable;
    this.buttons.table.disabled = inTable;
  };

  function enhance(root) {
    var areas = (root || document).querySelectorAll ? (root || document).querySelectorAll("textarea[data-md-editor]") : [];
    var mode = storedMode(storage());
    for (var i = 0; i < areas.length; i++) {
      var area = areas[i];
      if (area.getAttribute("data-md-ready")) continue;
      area.setAttribute("data-md-ready", "1");
      var ed = new Editor(area);
      if (mode === "rich") ed.setMode("rich", false);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { enhance(document); });
  } else {
    enhance(document);
  }
  document.addEventListener("htmx:load", function (event) {
    enhance(event.target && event.target.querySelectorAll ? event.target : document);
  });
})();
