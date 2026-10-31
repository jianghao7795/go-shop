import {
  coreCreateEditor, coreCreateToolbar, i18nAddResources,
  registerMenu, registerRenderElemConf, registerStyleHandler,
  registerElemToHtmlConf, registerStyleToHtmlHandler, registerPreParseHtmlConf,
  registerParseElemHtmlConf, registerParseStyleHtmlHandler,
  type IDomEditor, type IEditorConfig, type IModuleConf, type IToolbarConfig,
} from "@wangeditor/core";
import basicModules from "@wangeditor/basic-modules";
import listModule from "@wangeditor/list-module";
import tableModule from "@wangeditor/table-module";
import videoModule from "@wangeditor/video-module";
import uploadImageModule from "@wangeditor/upload-image-module";
import { wangEditorCodeHighlightModule, wangEditorCodeHighLightDecorate } from "@wangeditor/code-highlight";

// The editor package ships a prebundled JS entry. Register its public feature
// modules separately so Vite can split them without dropping editor capabilities.
const plugins: NonNullable<IModuleConf["editorPlugin"]>[] = [];
const modules: Partial<IModuleConf>[] = [...basicModules, listModule, tableModule, videoModule, uploadImageModule, wangEditorCodeHighlightModule];
for (const module of modules) {
  module.menus?.forEach(menu => registerMenu(menu));
  module.renderElems?.forEach(render => registerRenderElemConf(render));
  if (module.renderStyle) registerStyleHandler(module.renderStyle);
  module.elemsToHtml?.forEach(render => registerElemToHtmlConf(render));
  if (module.styleToHtml) registerStyleToHtmlHandler(module.styleToHtml);
  module.preParseHtml?.forEach(parse => registerPreParseHtmlConf(parse));
  module.parseElemsHtml?.forEach(parse => registerParseElemHtmlConf(parse));
  if (module.parseStyleHtml) registerParseStyleHtmlHandler(module.parseStyleHtml);
  if (module.editorPlugin) plugins.push(module.editorPlugin);
}
i18nAddResources("zh-CN", { editor: { more: "更多", justify: "对齐", indent: "缩进", image: "图片", video: "视频" } });
i18nAddResources("en", { editor: { more: "More", justify: "Align", indent: "Indent", image: "Image", video: "Video" } });

const hoverbarKeys: IEditorConfig["hoverbarKeys"] = {
  link: { menuKeys: ["editLink", "unLink", "viewLink"] },
  image: { menuKeys: ["imageWidth30", "imageWidth50", "imageWidth100", "editImage", "viewImageLink", "deleteImage"] },
  pre: { menuKeys: ["enter", "codeBlock", "codeSelectLang"] },
  table: { menuKeys: ["enter", "tableHeader", "tableFullWidth", "insertTableRow", "deleteTableRow", "insertTableCol", "deleteTableCol", "deleteTable"] },
  divider: { menuKeys: ["enter"] },
  video: { menuKeys: ["enter", "editVideoSize"] },
  text: { menuKeys: ["headerSelect", "insertLink", "bulletedList", "|", "bold", "through", "color", "bgColor", "clearStyle"] },
};

const toolbarKeys: IToolbarConfig["toolbarKeys"] = [
  "headerSelect", "blockquote", "|", "bold", "underline", "italic",
  { key: "group-more-style", title: "更多样式", menuKeys: ["through", "code", "sup", "sub", "clearStyle"] },
  "color", "bgColor", "|", "fontSize", "fontFamily", "lineHeight", "|",
  "bulletedList", "numberedList", "todo",
  { key: "group-justify", title: "对齐", menuKeys: ["justifyLeft", "justifyRight", "justifyCenter", "justifyJustify"] },
  { key: "group-indent", title: "缩进", menuKeys: ["indent", "delIndent"] },
  "|", "emotion", "insertLink",
  { key: "group-image", title: "图片", menuKeys: ["insertImage", "uploadImage"] },
  { key: "group-video", title: "视频", menuKeys: ["insertVideo", "uploadVideo"] },
  "insertTable", "codeBlock", "divider", "|", "undo", "redo", "|", "fullScreen",
];

export function createProductEditor(selector: HTMLElement, html: string, config: Partial<IEditorConfig>) {
  return coreCreateEditor({
    selector, html, plugins,
    config: { decorate: wangEditorCodeHighLightDecorate, ...config, hoverbarKeys: { ...hoverbarKeys, ...config.hoverbarKeys } },
  });
}

export function createProductToolbar(editor: IDomEditor, selector: HTMLElement) {
  return coreCreateToolbar(editor, { selector, config: { toolbarKeys } });
}
