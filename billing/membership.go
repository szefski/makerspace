package billing

import (
	"database/sql"
	"log"
	"time"
)

func (p *Profile) New_membership(is_student bool) *Invoice {
	var fee *Fee
	if is_student {
		fee = p.Find_fee("membership", "student")
	} else {
		fee = p.Find_fee("membership", "regular")
	}
	inv := p.New_recurring_bill(fee, p.member_id)
	if prorated := prorate_month(fee.Amount); prorated != 0 {
		description := fee.Description
		if prorated != fee.Amount {
			description += " (prorated)"
		}
		first_inv := p.New_invoice(p.member_id, prorated, description, fee)
		if txn := p.do_transaction(first_inv); txn == nil || !txn.Approved {
			//TODO: do_missed_payment
			//embed error
		}
	}
	return inv
}

func (p *Profile) Get_membership() *Invoice {
	var id int
	if err := p.db.QueryRow(
		"SELECT i.id "+
			"FROM invoice i "+
			"JOIN fee f "+
			"ON f.id = i.fee "+
			"WHERE i.member = $1 "+
			"	AND f.category = 'membership'"+
			"	AND (i.end_date > now() OR i.end_date IS NULL)",
		p.member_id).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		log.Panic(err)
	}
	return p.Billing.Get_bill(id)
}

//TODO: set end_date and regular membership invoice to start after
func (p *Profile) Change_to_student(grad_date time.Time) {
	invoice := p.Get_membership()
	date := invoice.Date
	p.Cancel_recurring_bill(invoice)
	invoice = p.New_recurring_bill(p.Find_fee("membership", "student"), p.member_id)
	p.set_invoice_start_date(invoice, date)
}

func (p *Profile) Change_from_student() {
	invoice := p.Get_membership()
	date := invoice.Date
	p.Cancel_recurring_bill(invoice)
	invoice = p.New_recurring_bill(p.Find_fee("membership", "regular"), p.member_id)
	p.set_invoice_start_date(invoice, date)
}

//TODO: Cancel storage and other makerspace-related invoices
//TODO: send card-cancellation e-mail to VITP
func (p *Profile) Cancel_membership() {
	invoice := p.Get_membership()
	p.Cancel_recurring_bill(invoice)
}
