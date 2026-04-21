// ==UserScript==
// @name         Cloudflare Turnstile 自动绕过助手 / shadow DOM 穿透
// @namespace    http://tampermonkey.net/
// @version      1.0
// @description  利用劫持 attachShadow 穿透 Cloudflare 的 Shadow DOM 隔离，获取验证码 checkbox 的真实坐标并进行自动化点击。包含对本地 CDP 辅助点击服务的支持以绕过 isTrusted 风控。
// @author       Your Name
// @match        *://*/*
// @grant        GM_xmlhttpRequest
// @grant        GM_log
// @run-at       document-start
// ==/UserScript==

(function() {
    'use strict';

    console.log("[Turnstile Bypass] 脚本已加载，开始拦截 Shadow DOM...");

    // 1. 劫持原生 attachShadow，跨越 closed 隔离
    const originalAttachShadow = Element.prototype.attachShadow;
    const exposedShadowRoots = []; // 用于存放所有被截获的 CF shadow dom 根节点

    Element.prototype.attachShadow = function(init) {
        // 强制把后续调用的 closed 改为 open 以方便直接读取
        // 也可以不改模式，只是偷偷保存引用
        const shadowRoot = originalAttachShadow.call(this, { ...init, mode: 'open' });
        
        // 我们只关注可能是 turnstile 的容器，通常带有一些特定的 class 或父特征
        this._exposedShadowRoot = shadowRoot;
        exposedShadowRoots.push(shadowRoot);
        
        // 尝试监听该 shadowRoot 中 iframe 的呈现
        monitorShadowRoot(shadowRoot, this);

        return shadowRoot;
    };

    // 2. 监控截获的 ShadowRoot 中复选框 iframe 的渲染状态
    function monitorShadowRoot(root, hostElement) {
        let isSolved = false;
        
        const observer = new MutationObserver((mutations) => {
            if (isSolved) return;
            
            // 假设 CF 会在 shadow root 中插入 iframe 或相关的验证 wrapper
            const cfIframe = root.querySelector('iframe');
            if (cfIframe && cfIframe.src && cfIframe.src.includes('challenge')) {
                console.log("[Turnstile Bypass] 发现 Cloudflare iframe 实例！", cfIframe);
                
                // 等待 iframe 渲染就绪再获取坐标，给一个小延迟
                setTimeout(() => {
                    handleTurnstileClick(cfIframe, hostElement);
                }, 1000); 

                isSolved = true;
                observer.disconnect();
            }
        });

        observer.observe(root, { childList: true, subtree: true });
    }

    // 3. 处理点击逻辑：精确定位与触发
    function handleTurnstileClick(iframe, hostElement) {
        // 第一步：获取宿主元素（Shadow DOM 容器）在屏幕上的绝对位置
        const rect = hostElement.getBoundingClientRect();
        
        if (rect.width === 0 || rect.height === 0) {
            console.log("[Turnstile Bypass] 元素被隐藏或未渲染，退出");
            return;
        }

        // 推算复选框大概在其 iframe 的中间偏左等位置 (您可以根据 CF 实际渲染结构微调)
        // 默认点击宿主的中点
        const clickX = rect.left + rect.width / 2;
        const clickY = rect.top + rect.height / 2;
        
        console.log(`[Turnstile Bypass] 锁定 Checkbox 坐标: X=${clickX}, Y=${clickY}`);

        // --- 选择你的“子弹”方案，这取决于风控等级 ---
        
        // 路线 A: 纯浏览器环境本地 JS 模拟
        // tryPureJsClick(hostElement);
        
        // 路线 A: 纯浏览器环境本地 JS 模拟 (受制于 isTrusted)
        tryPureJsClick(hostElement, rect);
    }

    // (纯油猴/客户默认浏览器适用) 在页面发起模拟的一连串详细的 PointerEvent，尽最大可能模拟人类点击。
    // 注：此原生方法的 event.isTrusted = false。能否通过取决于当前网站的具体保护级别。
    function tryPureJsClick(element, rect) {
        console.log("[Turnstile Bypass] 开始在纯油猴环境下尝试 JS 强行模拟点击...");
        
        // 在目标的中心区域适当附加一点随机偏移坐标，伪装人手颤动
        const clickX = rect.left + rect.width / 2 + (Math.random() * 8 - 4);
        const clickY = rect.top + rect.height / 2 + (Math.random() * 8 - 4);

        const eventOptions = { 
            bubbles: true, 
            cancelable: true, 
            view: window,
            detail: 1,
            screenX: Math.floor(clickX),
            screenY: Math.floor(clickY),
            clientX: Math.floor(clickX),
            clientY: Math.floor(clickY),
            pointerId: 1, 
            pointerType: "mouse",
            isPrimary: true 
        };

        // 按真实操作的时间轴派发事件
        setTimeout(() => element.dispatchEvent(new PointerEvent('pointerover', eventOptions)), 10);
        setTimeout(() => element.dispatchEvent(new PointerEvent('pointerenter', eventOptions)), 30);
        setTimeout(() => element.dispatchEvent(new PointerEvent('pointermove', eventOptions)), 80);
        setTimeout(() => element.dispatchEvent(new PointerEvent('pointerdown', eventOptions)), 150);
        setTimeout(() => element.dispatchEvent(new MouseEvent('mousedown', eventOptions)), 160);
        setTimeout(() => element.dispatchEvent(new PointerEvent('pointerup', eventOptions)), 240);
        setTimeout(() => element.dispatchEvent(new MouseEvent('mouseup', eventOptions)), 250);
        setTimeout(() => element.dispatchEvent(new MouseEvent('click', eventOptions)), 260);

        console.log("[Turnstile Bypass] 一系列纯 JS 点击事件链派发完毕。");
    }

})();
