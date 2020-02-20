if err := r.Ops.Connected.QueryRow(`SELECT binding_id FROM share_binding where share_instance_pid = $1 and binding_status = $2`,
instance.shareInstancePid, StatusExist).Scan(&bindingId); err != nil {
if err == sql.ErrNoRows {
	logger.Recorder.Println("No exist binding.")
} else {
	errString := err.Error()
	return osb.HTTPStatusCodeError{
		StatusCode:   http.StatusInternalServerError,
		ErrorMessage: &errString,
	}
}
} else { //binding still exist
logger.RecorderErr.Println("find bindind:", bindingId)
errString := "Cannot delete service instance, service keys and bindings must first be deleted."
return osb.HTTPStatusCodeError{
	StatusCode:   http.StatusBadRequest,
	ErrorMessage: &errString,
}
}