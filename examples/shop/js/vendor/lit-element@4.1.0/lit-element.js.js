/**
 * Bundled by jsDelivr using Rollup v4.62.2 and esbuild v0.28.1.
 * Original file: /npm/lit-element@4.1.0/lit-element.js
 *
 * Do NOT use SRI with dynamically generated files! More information: https://www.jsdelivr.com/using-sri-with-dynamic-files
 */
import{ReactiveElement as s}from"../@lit/reactive-element@2.0.4.js";export*from"../@lit/reactive-element@2.0.4.js";import{render as o,noChange as i}from"../lit-html@3.3.3.js";export*from"../lit-html@3.3.3.js";class t extends s{constructor(){super(...arguments),this.renderOptions={host:this},this.o=void 0}createRenderRoot(){const e=super.createRenderRoot();return this.renderOptions.renderBefore??=e.firstChild,e}update(e){const r=this.render();this.hasUpdated||(this.renderOptions.isConnected=this.isConnected),super.update(e),this.o=o(r,this.renderRoot,this.renderOptions)}connectedCallback(){super.connectedCallback(),this.o?.setConnected(!0)}disconnectedCallback(){super.disconnectedCallback(),this.o?.setConnected(!1)}render(){return i}}t._$litElement$=!0,t.finalized=!0,globalThis.litElementHydrateSupport?.({LitElement:t});const l=globalThis.litElementPolyfillSupport;l?.({LitElement:t});const d={_$AK:(n,e,r)=>{n._$AK(e,r)},_$AL:n=>n._$AL};(globalThis.litElementVersions??=[]).push("4.1.0");export{t as LitElement,d as _$LE};
