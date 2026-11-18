/* DSM 7.1 application window. Uses the DSM ExtJS application framework. */
Ext.namespace("SYNO.SDS.h5uSynoWake");

Ext.define("SYNO.SDS.h5uSynoWake.Application", {
    extend: "SYNO.SDS.AppInstance",
    appWindowName: "SYNO.SDS.h5uSynoWake.MainWindow",
    constructor: function () {
        this.callParent(arguments);
    }
});

Ext.define("SYNO.SDS.h5uSynoWake.MainWindow", {
    extend: "SYNO.SDS.AppWindow",
    constructor: function (options) {
        this.appInstance = options.appInstance;
        SYNO.SDS.h5uSynoWake.MainWindow.superclass.constructor.call(this, Ext.apply({
            title: "SynoWake",
            layout: "fit",
            cls: "syno-app-win",
            width: 920,
            height: 660,
            minWidth: 650,
            minHeight: 440,
            resizable: true,
            maximizable: true,
            minimizable: true,
            html: '<iframe class="synowake-frame" title="SynoWake" src="/webman/3rdparty/h5uSynoWake/index.html?v=1.1.2-0020" ' +
                'style="display:block;width:100%;height:100%;border:0" ' +
                'referrerpolicy="same-origin"></iframe>'
        }, options));
    },
    afterRender: function () {
        SYNO.SDS.h5uSynoWake.MainWindow.superclass.afterRender.apply(this, arguments);
        const frame = this.body.dom.querySelector('.synowake-frame');
        if (!frame) return;

        let frameDocument = null;
        const activate = event => {
            if (event.button === 0) {
                // DSM needs a target in its desktop document, not inside the iframe.
                this.toFront({getTarget: () => frame});
            }
        };
        const bindFrame = () => {
            frameDocument?.removeEventListener('pointerdown', activate, true);
            frameDocument = null;
            try {
                frameDocument = frame.contentDocument;
                frameDocument?.addEventListener('pointerdown', activate, true);
            } catch (_) {
                // A redirect or navigation may temporarily make the iframe inaccessible.
            }
        };
        frame.addEventListener('load', bindFrame);
        bindFrame();
        this.removeFrameListeners = () => {
            frame.removeEventListener('load', bindFrame);
            frameDocument?.removeEventListener('pointerdown', activate, true);
            frameDocument = null;
        };
    },
    beforeDestroy: function () {
        this.removeFrameListeners?.();
        SYNO.SDS.h5uSynoWake.MainWindow.superclass.beforeDestroy.apply(this, arguments);
    }
});
