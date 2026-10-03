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
            html: '<iframe title="SynoWake" src="/webman/3rdparty/h5uSynoWake/index.html?v=1.0.0-0017" ' +
                'style="display:block;width:100%;height:100%;border:0" ' +
                'referrerpolicy="same-origin"></iframe>'
        }, options));
    }
});
