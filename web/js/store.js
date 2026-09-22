/* global app */
(function (window) {
    "use strict";

    var API_BASE = "/todos";

    /**
     * request 呼叫後端 API，統一解開 {code, msg, data} envelope。
     *
     * code !== 0（含網路錯誤）一律 alert 顯示 msg，並讓呼叫端的 promise reject，
     * 呼叫端不需要重複處理錯誤訊息。
     */
    function request(method, url, body) {
        var options = { method: method, headers: {} };

        if (body !== undefined) {
            options.headers["Content-Type"] = "application/json";
            options.body = JSON.stringify(body);
        }

        return fetch(url, options)
            .then(function (res) {
                return res.json().then(function (payload) {
                    if (payload.code !== "E000")
                        throw new Error(payload.msg || "請求失敗");

                    return payload.data;
                });
            })
            .catch(function (err) {
                window.alert(err.message || "網路連線失敗，請稍後再試");
                throw err;
            });
    }

    /**
     * Store 打後端 API，對外維持跟原本 localStorage 版一樣的
     * find / findAll / save / remove / drop 介面，Model 不必知道底層換了實作。
     */
    function Store(name, callback) {
        callback = callback || function () {};
        callback.call(this, []);
    }

    /**
     * 依 query 篩選 todo。目前只會被 Controller 用 {completed: bool} 或
     * {id: number} 這兩種形狀呼叫。
     */
    Store.prototype.find = function (query, callback) {
        if (!callback)
            return;

        var self = this;

        if (Object.prototype.hasOwnProperty.call(query, "id")) {
            request("GET", `${API_BASE}/${query.id}`)
                .then(function (todo) {
                    callback.call(self, [todo]);
                })
                .catch(function () {});
            return;
        }

        var status = query.completed ? "completed" : "active";
        request("GET", `${API_BASE}?status=${status}&page_size=200`)
            .then(function (items) {
                callback.call(self, items);
            })
            .catch(function () {});
    };

    /**
     * 取回全部未刪除的 todo。page_size 給上限 200——這個專案的量體下
     * 不會需要分頁 UI，超過 200 筆之後看不到的部分之後有需要再處理。
     */
    Store.prototype.findAll = function (callback) {
        callback = callback || function () {};
        var self = this;

        request("GET", `${API_BASE}?status=all&page_size=200`)
            .then(function (items) {
                callback.call(self, items);
            })
            .catch(function () {});
    };

    /**
     * 沒帶 id 是新增（POST），帶 id 是部分更新（PATCH /todos/:id）。
     * 原版呼叫端一律不理會 callback 帶回來的值，所以這裡回傳什麼都不影響行為。
     */
    Store.prototype.save = function (updateData, callback, id) {
        callback = callback || function () {};
        var self = this;

        if (id) {
            request("PATCH", `${API_BASE}/${id}`, updateData)
                .then(function (todo) {
                    callback.call(self, [todo]);
                })
                .catch(function () {});
        } else {
            request("POST", API_BASE, { title: updateData.title })
                .then(function (todo) {
                    callback.call(self, [todo]);
                })
                .catch(function () {});
        }
    };

    /** 軟刪除單筆。 */
    Store.prototype.remove = function (id, callback) {
        callback = callback || function () {};
        var self = this;

        request("DELETE", `${API_BASE}/${id}`)
            .then(function () {
                callback.call(self, []);
            })
            .catch(function () {});
    };

    /**
     * 原版語意是「清空整個 storage」，但畫面上沒有對應的按鈕會呼叫它
     * （唯一的批次刪除是「清除已完成」，走 removeCompleted）。後端也沒有
     * 「刪除全部」的端點，這裡就不接了，保留介面但不做事。
     */
    Store.prototype.drop = function (callback) {
        callback = callback || function () {};
        callback.call(this, []);
    };

    /**
     * 全部設為完成 / 取消完成，一次 PATCH /todos 打完，不逐筆發請求。
     */
    Store.prototype.completeAll = function (completed, callback) {
        callback = callback || function () {};
        var self = this;

        request("PATCH", API_BASE, { completed: completed })
            .then(function (data) {
                callback.call(self, data);
            })
            .catch(function () {});
    };

    /**
     * 清除所有已完成，一次 DELETE /todos?status=completed 打完。
     */
    Store.prototype.removeCompleted = function (callback) {
        callback = callback || function () {};
        var self = this;

        request("DELETE", `${API_BASE}?status=completed`)
            .then(function (data) {
                callback.call(self, data);
            })
            .catch(function () {});
    };

    /**
     * footer 的統計數字打獨立的 /todos-summary，不透過 /todos 列表端點：
     * 後端不用為了三個數字多跑一次帶篩選/分頁條件的列表查詢。
     */
    Store.prototype.getCount = function (callback) {
        callback = callback || function () {};
        var self = this;

        request("GET", "/todos-summary")
            .then(function (summary) {
                callback.call(self, summary);
            })
            .catch(function () {});
    };

    // Export to window
    window.app = window.app || {};
    window.app.Store = Store;
})(window);
