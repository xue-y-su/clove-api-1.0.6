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

import React from 'react';
import { Link } from 'react-router-dom';
import { Button, Dropdown, Tooltip } from '@douyinfe/semi-ui';
import { Compass } from 'lucide-react';
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isCompactHeader,
  isLoading,
  userState,
  pricingRequireAuth,
  navigationLabel,
}) => {
  const resolveTargetPath = (link) => {
    if (link.itemKey === 'console' && !userState.user) return '/login';
    if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
      return '/login';
    }
    return link.to;
  };

  const renderMobileLink = (link) =>
    link.isExternal ? (
      <a
        href={link.externalLink}
        target='_blank'
        rel='noopener noreferrer'
        className='block w-full px-1 py-1'
      >
        {link.text}
      </a>
    ) : (
      <Link to={resolveTargetPath(link)} className='block w-full px-1 py-1'>
        {link.text}
      </Link>
    );

  const MobileNavigationMenu = () => (
    <Dropdown
      position='bottom'
      render={
        <Dropdown.Menu>
          {mainNavLinks.map((link) => (
            <Dropdown.Item key={link.itemKey}>
              {renderMobileLink(link)}
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      }
    >
      <Tooltip content={navigationLabel}>
        <Button
          icon={<Compass size={18} />}
          aria-label={navigationLabel}
          theme='borderless'
          type='tertiary'
          className='!p-2 !text-current'
        />
      </Tooltip>
    </Dropdown>
  );

  const renderNavLinks = () => {
    const baseClasses =
      'flex-shrink-0 flex items-center gap-1 font-semibold rounded-md transition-all duration-200 ease-in-out';
    const hoverClasses = 'hover:text-semi-color-primary';
    const spacingClasses = isMobile ? 'p-1' : 'p-2';

    const commonLinkClasses = `${baseClasses} ${spacingClasses} ${hoverClasses}`;

    return mainNavLinks.map((link) => {
      const linkContent = <span>{link.text}</span>;

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className={commonLinkClasses}
          >
            {linkContent}
          </a>
        );
      }

      return (
        <Link
          key={link.itemKey}
          to={resolveTargetPath(link)}
          className={commonLinkClasses}
        >
          {linkContent}
        </Link>
      );
    });
  };

  return (
    <nav className='flex min-w-0 flex-1 items-center justify-center gap-1 lg:gap-2 mx-1 md:mx-4 whitespace-nowrap'>
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={4}
        width={60}
        height={16}
        isMobile={isMobile}
      >
        {isCompactHeader ? <MobileNavigationMenu /> : renderNavLinks()}
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
