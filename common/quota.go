package common

func GetTrustQuota() int {
	return QuotaFromFloat(10 * QuotaPerUnit)
}
