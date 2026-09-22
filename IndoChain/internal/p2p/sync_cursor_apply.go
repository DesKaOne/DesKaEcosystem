package p2p

// ApplyResponseFromCursor applies one synchronization batch against the
// cursor's known parent and returns the advanced cursor only on success.
func (c *SyncCoordinator) ApplyResponseFromCursor(cursor SyncCursor, req BlockRequest, resp BlockResponse) (SyncCursor, error) {
	if err := ValidateSyncCursorParent(cursor, req, resp); err != nil {
		return cursor, err
	}

	progress, err := c.ApplyResponseWithProgress(req, resp, cursor.BlockHash)
	if err != nil {
		return cursor, err
	}

	next, err := cursor.Advance(progress)
	if err != nil {
		return cursor, err
	}
	return next, nil
}
