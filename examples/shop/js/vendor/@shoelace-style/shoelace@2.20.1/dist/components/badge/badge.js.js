/**
 * Bundled by jsDelivr using Rollup v4.62.2 and esbuild v0.28.1.
 * Original file: /npm/@shoelace-style/shoelace@2.20.1/dist/components/badge/badge.js
 *
 * Do NOT use SRI with dynamically generated files! More information: https://www.jsdelivr.com/using-sri-with-dynamic-files
 */
import{css as u,LitElement as m,html as y}from"../../../../../lit@3.2.1.js";import{property as i}from"../../../../../lit@3.2.1/decorators.js.js";import{classMap as _}from"../../../../../lit@3.2.1/directives/class-map.js.js";var w=u`
  :host {
    display: inline-flex;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: max(12px, 0.75em);
    font-weight: var(--sl-font-weight-semibold);
    letter-spacing: var(--sl-letter-spacing-normal);
    line-height: 1;
    border-radius: var(--sl-border-radius-small);
    border: solid 1px var(--sl-color-neutral-0);
    white-space: nowrap;
    padding: 0.35em 0.6em;
    user-select: none;
    -webkit-user-select: none;
    cursor: inherit;
  }

  /* Variant modifiers */
  .badge--primary {
    background-color: var(--sl-color-primary-600);
    color: var(--sl-color-neutral-0);
  }

  .badge--success {
    background-color: var(--sl-color-success-600);
    color: var(--sl-color-neutral-0);
  }

  .badge--neutral {
    background-color: var(--sl-color-neutral-600);
    color: var(--sl-color-neutral-0);
  }

  .badge--warning {
    background-color: var(--sl-color-warning-600);
    color: var(--sl-color-neutral-0);
  }

  .badge--danger {
    background-color: var(--sl-color-danger-600);
    color: var(--sl-color-neutral-0);
  }

  /* Pill modifier */
  .badge--pill {
    border-radius: var(--sl-border-radius-pill);
  }

  /* Pulse modifier */
  .badge--pulse {
    animation: pulse 1.5s infinite;
  }

  .badge--pulse.badge--primary {
    --pulse-color: var(--sl-color-primary-600);
  }

  .badge--pulse.badge--success {
    --pulse-color: var(--sl-color-success-600);
  }

  .badge--pulse.badge--neutral {
    --pulse-color: var(--sl-color-neutral-600);
  }

  .badge--pulse.badge--warning {
    --pulse-color: var(--sl-color-warning-600);
  }

  .badge--pulse.badge--danger {
    --pulse-color: var(--sl-color-danger-600);
  }

  @keyframes pulse {
    0% {
      box-shadow: 0 0 0 0 var(--pulse-color);
    }
    70% {
      box-shadow: 0 0 0 0.5rem transparent;
    }
    100% {
      box-shadow: 0 0 0 0 transparent;
    }
  }
`,x=u`
  :host {
    box-sizing: border-box;
  }

  :host *,
  :host *::before,
  :host *::after {
    box-sizing: inherit;
  }

  [hidden] {
    display: none !important;
  }
`,g=Object.defineProperty,v=Object.getOwnPropertySymbols,E=Object.prototype.hasOwnProperty,P=Object.prototype.propertyIsEnumerable,f=r=>{throw TypeError(r)},h=(r,e,t)=>e in r?g(r,e,{enumerable:!0,configurable:!0,writable:!0,value:t}):r[e]=t,k=(r,e)=>{for(var t in e||(e={}))E.call(e,t)&&h(r,t,e[t]);if(v)for(var t of v(e))P.call(e,t)&&h(r,t,e[t]);return r},n=(r,e,t,s)=>{for(var a=void 0,l=r.length-1,p;l>=0;l--)(p=r[l])&&(a=p(e,t,a)||a);return a&&g(e,t,a),a},b=(r,e,t)=>e.has(r)||f("Cannot "+t),O=(r,e,t)=>(b(r,e,"read from private field"),e.get(r)),C=(r,e,t)=>e.has(r)?f("Cannot add the same private member more than once"):e instanceof WeakSet?e.add(r):e.set(r,t),S=(r,e,t,s)=>(b(r,e,"write to private field"),e.set(r,t),t),d,c=class extends m{constructor(){super(),C(this,d,!1),this.initialReflectedProperties=new Map,Object.entries(this.constructor.dependencies).forEach(([r,e])=>{this.constructor.define(r,e)})}emit(r,e){const t=new CustomEvent(r,k({bubbles:!0,cancelable:!1,composed:!0,detail:{}},e));return this.dispatchEvent(t),t}static define(r,e=this,t={}){const s=customElements.get(r);if(!s){try{customElements.define(r,e,t)}catch{customElements.define(r,class extends e{},t)}return}let a=" (unknown version)",l=a;"version"in e&&e.version&&(a=" v"+e.version),"version"in s&&s.version&&(l=" v"+s.version),!(a&&l&&a===l)&&console.warn(`Attempted to register <${r}>${a}, but <${r}>${l} has already been registered.`)}attributeChangedCallback(r,e,t){O(this,d)||(this.constructor.elementProperties.forEach((s,a)=>{s.reflect&&this[a]!=null&&this.initialReflectedProperties.set(a,this[a])}),S(this,d,!0)),super.attributeChangedCallback(r,e,t)}willUpdate(r){super.willUpdate(r),this.initialReflectedProperties.forEach((e,t)=>{r.has(t)&&this[t]==null&&(this[t]=e)})}};d=new WeakMap,c.version="2.20.1",c.dependencies={},n([i()],c.prototype,"dir"),n([i()],c.prototype,"lang");var o=class extends c{constructor(){super(...arguments),this.variant="primary",this.pill=!1,this.pulse=!1}render(){return y`
      <span
        part="base"
        class=${_({badge:!0,"badge--primary":this.variant==="primary","badge--success":this.variant==="success","badge--neutral":this.variant==="neutral","badge--warning":this.variant==="warning","badge--danger":this.variant==="danger","badge--pill":this.pill,"badge--pulse":this.pulse})}
        role="status"
      >
        <slot></slot>
      </span>
    `}};o.styles=[x,w],n([i({reflect:!0})],o.prototype,"variant"),n([i({type:Boolean,reflect:!0})],o.prototype,"pill"),n([i({type:Boolean,reflect:!0})],o.prototype,"pulse");var R=o;o.define("sl-badge");export{R as default};
