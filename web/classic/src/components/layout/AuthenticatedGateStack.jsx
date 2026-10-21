/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useCallback, useContext, useEffect, useState } from 'react';
import ComplianceGate from '../compliance/ComplianceGate';
import SourceAttributionGate from '../source-attribution/SourceAttributionGate';
import { UserContext } from '../../context/User';
import GlobalAnnouncementModal from './GlobalAnnouncementModal';
import GateLoadError from './GateLoadError';
import { getAuthenticatedGateAccess } from './authenticated-gates';

const AuthenticatedGateStack = () => {
  const [userState] = useContext(UserContext);
  const userId = userState?.user?.id;
  const [complianceResolution, setComplianceResolution] = useState('loading');
  const [sourceAttributionResolution, setSourceAttributionResolution] =
    useState('loading');
  const [complianceError, setComplianceError] = useState(null);
  const [sourceAttributionError, setSourceAttributionError] = useState(null);
  const [complianceRetryToken, setComplianceRetryToken] = useState(0);
  const [sourceAttributionRetryToken, setSourceAttributionRetryToken] =
    useState(0);
  const access = getAuthenticatedGateAccess({
    signedIn: Boolean(userId),
    compliance: complianceResolution,
    sourceAttribution: sourceAttributionResolution,
  });

  useEffect(() => {
    setComplianceResolution('loading');
    setSourceAttributionResolution('loading');
    setComplianceError(null);
    setSourceAttributionError(null);
  }, [userId]);

  const handleComplianceResolution = useCallback((resolution, error) => {
    setComplianceResolution(resolution);
    setComplianceError(resolution === 'error' ? error : null);
  }, []);

  const handleSourceAttributionResolution = useCallback((resolution, error) => {
    setSourceAttributionResolution(resolution);
    setSourceAttributionError(resolution === 'error' ? error : null);
  }, []);

  const retryFailedGate = () => {
    if (access.failed === 'compliance') {
      setComplianceResolution('loading');
      setComplianceError(null);
      setComplianceRetryToken((current) => current + 1);
      return;
    }
    if (access.failed === 'sourceAttribution') {
      setSourceAttributionResolution('loading');
      setSourceAttributionError(null);
      setSourceAttributionRetryToken((current) => current + 1);
    }
  };

  if (!userId) return null;

  return (
    <>
      <ComplianceGate
        queryEnabled={Boolean(userId)}
        displayEnabled={access.compliance}
        retryToken={complianceRetryToken}
        onResolutionChange={handleComplianceResolution}
      />
      <SourceAttributionGate
        queryEnabled={Boolean(userId)}
        displayEnabled={access.sourceAttribution}
        retryToken={sourceAttributionRetryToken}
        onResolutionChange={handleSourceAttributionResolution}
      />
      <GlobalAnnouncementModal enabled={access.announcement} />
      {access.failed ? (
        <GateLoadError
          gate={access.failed}
          error={
            access.failed === 'compliance'
              ? complianceError
              : sourceAttributionError
          }
          onRetry={retryFailedGate}
        />
      ) : null}
    </>
  );
};

export default AuthenticatedGateStack;
