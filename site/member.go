package site

import (
	"net/http"
)

func (h *Http_server) member_handler() {
	h.sso_handler()
	h.billing_handler()
	h.tools_handler()
	h.storage_handler()
	h.mux.HandleFunc("/member", func(w http.ResponseWriter, r *http.Request) {
		p := h.new_page("dashboard", "Dashboard", w, r)
		if !p.must_authenticate() {
			return
		}
		p.write_template()
	})
}

func (h *Http_server) tools_handler() {
	h.mux.HandleFunc("/member/tools", func(w http.ResponseWriter, r *http.Request) {
		p := h.new_page("tools", "Tools", w, r)
		if !p.must_authenticate() {
			return
		}
		p.write_template()
	})
}

func (h *Http_server) storage_handler() {
	h.mux.HandleFunc("/member/storage", func(w http.ResponseWriter, r *http.Request) {
		p := h.new_page("storage", "Storage", w, r)
		if !p.must_authenticate() {
			return
		}
		p.Field["fees"] = p.Fees
		p.Field["wall_storage"] = p.Storage["storage_wall"]
		p.Field["hall_lockers"] = p.Storage["storage_hall-locker"]
		p.Field["bathroom_lockers"] = p.Storage["storage_bathroom-locker"]
		p.write_template()
	})
}

//preferences
// sync_sso
